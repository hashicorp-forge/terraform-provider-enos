package transport

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
)

// CopyWriter a function that can write the stdin for a copy request.
type CopyWriter func(errorC chan error)

// TarCopyWriter creates a new CopyWriter that uses tar to copy a file.
func TarCopyWriter(src Copyable, dst string, stdin io.WriteCloser) CopyWriter {
	return func(errorC chan error) {
		defer stdin.Close()

		writer := tar.NewWriter(stdin)
		defer writer.Close()

		err := writer.WriteHeader(&tar.Header{
			Name: filepath.Base(dst),
			Mode: 0o644,
			Size: src.Size(),
		})
		if err != nil {
			errorC <- err
			return
		}

		_, err = io.Copy(writer, src)
		errorC <- err
	}
}

// ExecError An exec error is a wrapper error that all transport implementations should return if the
// exec failed with an exit code.
type ExecError struct {
	merr     *multierror.Error
	exitCode int
}

// NewExecError creates a new ExecError with the provided exitCode and wrapped error.
func NewExecError(err error, exitCode int) error {
	merr := &multierror.Error{}
	merr = multierror.Append(merr, err)

	return &ExecError{
		merr:     merr,
		exitCode: exitCode,
	}
}

// Error returns the wrapped error.
func (e *ExecError) Error() string {
	if len(e.merr.Errors) == 1 {
		return e.merr.Unwrap().Error()
	}

	return e.merr.Error()
}

func (e *ExecError) Unwrap() error {
	if len(e.merr.Errors) == 1 {
		return e.merr.Unwrap()
	}

	return e.merr.ErrorOrNil()
}

func (e *ExecError) Append(err error) {
	e.merr = multierror.Append(e.merr, err)
}

func (e *ExecError) ExitCode() int {
	return e.exitCode
}

// ExecRequest can be used to execute a remote command.
type ExecRequest interface {
	// Exec executes a remote command
	Exec(ctx context.Context) *ExecResponse
	// Streams retrieves the IO streams for the ExecRequest
	Streams() *ExecStreams
}

// ExecStreams The IO streams associated with an ExecRequest.
type ExecStreams struct {
	stdoutReader io.Reader
	stdoutWriter io.WriteCloser

	stderrReader io.Reader
	stderrWriter io.WriteCloser

	stdin       io.Reader
	stdinWriter io.WriteCloser
}

func NewExecStreams(stdin bool) *ExecStreams {
	stdout, stdOutWriter := io.Pipe()
	stderr, stdErrWriter := io.Pipe()

	streams := &ExecStreams{
		stdoutReader: stdout,
		stdoutWriter: stdOutWriter,
		stderrReader: stderr,
		stderrWriter: stdErrWriter,
	}

	if stdin {
		stdIn, stdInWriter := io.Pipe()
		streams.stdin = stdIn
		streams.stdinWriter = stdInWriter
	}

	return streams
}

func (e *ExecStreams) Stdout() io.Reader {
	return e.stdoutReader
}

func (e *ExecStreams) StdoutWriter() io.WriteCloser {
	return e.stdoutWriter
}

func (e *ExecStreams) Stderr() io.Reader {
	return e.stderrReader
}

func (e *ExecStreams) StderrWriter() io.WriteCloser {
	return e.stderrWriter
}

func (e *ExecStreams) Stdin() io.Reader {
	return e.stdin
}

func (e *ExecStreams) StdinWriter() io.WriteCloser {
	return e.stdinWriter
}

// Close closes all the streams and returns the aggregate error if any error occurred while closing
// any of the streams.
func (e *ExecStreams) Close() error {
	merr := &multierror.Error{}

	merr = multierror.Append(merr, e.stdoutWriter.Close())
	merr = multierror.Append(merr, e.stderrWriter.Close())
	if e.stdinWriter != nil {
		merr = multierror.Append(merr, e.stdinWriter.Close())
	}

	return merr.ErrorOrNil()
}

// ExecResponse a Response that a transport exec request should return.
type ExecResponse struct {
	Stdout  io.Reader
	Stderr  io.Reader
	ExecErr chan error
}

func NewExecResponse() *ExecResponse {
	return &ExecResponse{
		ExecErr: make(chan error, 1),
	}
}

// WaitForResults waits for the execution to finish and returns the stdout, stderr and the execution error
// if there is any.
func (e *ExecResponse) WaitForResults() (stdout, stderr string, err error) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	captureOutput := func(writer io.StringWriter, reader io.Reader, errC chan error, stream string) {
		// the stream reader can be nil, if the exec call fails early, so we need to guard against that
		if reader != nil {
			scanner := bufio.NewScanner(reader)
			scanner.Split(bufio.ScanBytes) // without this the scanner will swallow new lines
			for scanner.Scan() {
				text := scanner.Text()
				_, execErr := writer.WriteString(text)
				if execErr != nil {
					errC <- fmt.Errorf("failed to write exec %s, due to %v", stream, execErr)
					break
				}
			}
		}
		wg.Done()
	}

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	streamWriteErrC := make(chan error, 2)
	go captureOutput(stdoutBuf, e.Stdout, streamWriteErrC, "stdout")
	go captureOutput(stderrBuf, e.Stderr, streamWriteErrC, "stderr")

	execErr := <-e.ExecErr

	wg.Wait()
	close(streamWriteErrC)

	writeErrs := &multierror.Error{}
	for streamErr := range streamWriteErrC {
		writeErrs = multierror.Append(writeErrs, streamErr)
	}

	stdout = strings.TrimSuffix(stdoutBuf.String(), "\n")
	stderr = strings.TrimSuffix(stderrBuf.String(), "\n")

	if execErr != nil {
		switch e := execErr.(type) {
		case *ExecError:
			e.Append(writeErrs)
			err = e
		default:
			// in the case that the exec error is not a transport.ExecError we create a new multierror
			// here and append the exec error first then the other errors.
			allErrors := &multierror.Error{}
			allErrors = multierror.Append(allErrors, execErr, writeErrs)
			err = allErrors.ErrorOrNil()
		}
	} else {
		err = writeErrs.ErrorOrNil()
	}

	return stdout, stderr, err
}

// Copy executes a copy request.
func Copy(ctx context.Context, copyWriter CopyWriter, dst string, request ExecRequest) error {
	writeErrC := make(chan error, 1)

	go copyWriter(writeErrC)

	_, stderr, execErr := Run(ctx, request)

	// merr is used to capture the write error and the stderr
	merr := &multierror.Error{}

	if err := <-writeErrC; err != nil {
		merr = multierror.Append(merr, err)
	}

	if len(stderr) > 0 {
		merr = multierror.Append(merr, fmt.Errorf("failed to copy to dst: [%s], due to: [%s]", dst, stderr))
	}

	if execErr != nil {
		switch e := execErr.(type) {
		case *ExecError:
			e.Append(merr)
			return e
		default:
			// in the case that the exec error is not a transport.ExecError we create a new multierror
			// here and append the exec error first then the other errors.
			allErrors := &multierror.Error{}
			allErrors = multierror.Append(allErrors, execErr, merr)

			return allErrors.ErrorOrNil()
		}
	}

	return merr.ErrorOrNil()
}

// Stream executes a streaming remote request.
func Stream(ctx context.Context, request ExecRequest) (stdout io.Reader, stderr io.Reader, errC chan error) {
	response := request.Exec(ctx)

	stdout = response.Stdout
	stderr = response.Stderr
	errC = response.ExecErr

	return stdout, stderr, errC
}

// Run executes a remote exec and blocks waiting for the results.
func Run(ctx context.Context, request ExecRequest) (stdout string, stderr string, err error) {
	return request.Exec(ctx).WaitForResults()
}
