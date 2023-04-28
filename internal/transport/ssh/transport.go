package ssh

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	xssh "golang.org/x/crypto/ssh"

	"github.com/hashicorp/go-multierror"

	it "github.com/hashicorp/enos-provider/internal/transport"
)

// Opt is a functional option for the SSH transport.
type Opt func(*transport)

type transport struct {
	client *client
	ctx    context.Context
}

var _ it.Transport = (*transport)(nil)

// New takes zero or more functional options and return a new transport.
func New(opts ...Opt) (it.Transport, error) {
	t := &transport{
		client: &client{
			clientConfig: &xssh.ClientConfig{},
			transportCfg: &transportCfg{
				port: "22",
			},
		},
		ctx: context.Background(),
	}
	for _, opt := range opts {
		opt(t)
	}

	return t, t.client.init(t.ctx)
}

// WithUser sets the user.
func WithUser(u string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.user = u
	}
}

// WithHost sets the host.
func WithHost(h string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.host = h
	}
}

// WithKey sets the key.
func WithKey(k string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.key = k
	}
}

// WithKeyPath sets the key path.
func WithKeyPath(p string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.keyPath = p
	}
}

// WithPassphrase sets the key passphrase.
func WithPassphrase(p string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.passphrase = p
	}
}

// WithPassphrasePath sets the key passphrase path.
func WithPassphrasePath(p string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.passphrasePath = p
	}
}

// WithPassword sets the password.
func WithPassword(p string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.password = p
	}
}

// WithPort sets the port.
func WithPort(p string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.port = p
	}
}

// WithContext sets the context to use when initializing the resources.
func WithContext(ctx context.Context) func(*transport) {
	return func(t *transport) {
		t.ctx = ctx
	}
}

// Copy copies the source to the destination using the given SSH configuration
// options.
func (t *transport) Copy(ctx context.Context, src it.Copyable, dst string) (err error) {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	session, cleanup, err := t.client.newSession(ctx)
	if err != nil {
		return err
	}
	defer func() { err = cleanup() }()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("creating SSH STDIN pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating SSH STDOUT pipe: %w", err)
	}
	bufferedStdout := bufio.NewReader(stdout)

	checkSCPStdout := func() error {
		code, err := bufferedStdout.ReadByte()
		if err != nil {
			return fmt.Errorf("reading SCP session STDOUT: %w", err)
		}

		if code != 0 {
			msg, _, err := bufferedStdout.ReadLine()
			if err != nil {
				return fmt.Errorf("reading SCP sesssion error message: %w", err)
			}

			return fmt.Errorf("running SCP session command: %s", string(msg))
		}

		return nil
	}

	errC := make(chan error, 1)
	doneC := make(chan bool)

	copyFile := func() {
		defer stdin.Close()

		_, err := fmt.Fprintln(stdin, "C0644", src.Size(), filepath.Base(dst))
		if err != nil {
			errC <- fmt.Errorf("writing file header: %w", err)
			return
		}

		err = checkSCPStdout()
		if err != nil {
			errC <- fmt.Errorf("writing file header: %w", err)
			return
		}

		if src.Size() > 0 {
			_, err = io.Copy(stdin, src)
			if err != nil {
				errC <- fmt.Errorf("writing file: %w", err)
				return
			}
		}

		_, err = fmt.Fprint(stdin, "\x00")
		if err != nil {
			errC <- fmt.Errorf("writing end of file: %w", err)
			return
		}

		err = checkSCPStdout()
		if err != nil {
			errC <- fmt.Errorf("writing end of file: %w", err)
			return
		}

		errC <- nil
	}

	waitForCommandToFinish := func() {
		err = session.Wait()
		if err != nil {
			errC <- handleExecErr(err)
			return
		}

		doneC <- true
	}

	go copyFile()

	err = session.Run(fmt.Sprintf("scp -tr %s", dst))
	if err != nil {
		return fmt.Errorf("starting scp: %w", err)
	}

	go waitForCommandToFinish()

	select {
	case <-ctx.Done():
		err = ctx.Err()
		return err
	case err = <-errC:
		return err
	case <-doneC:
		return nil
	}
}

// Stream runs the given command and returns readers for STDOUT and STDERR and
// an err channel that will either contain one error or nil. The Stream request is considered complete
// when the error channel yields either an error or nil.
func (t *transport) Stream(ctx context.Context, cmd it.Command) (stdout, stderr io.Reader, errC chan error) {
	var err error
	errC = make(chan error, 3)

	select {
	case <-ctx.Done():
		errC <- ctx.Err()
		return stdout, stderr, errC
	default:
	}

	session, cleanup, err := t.client.newSession(ctx)
	if err != nil {
		errC <- err
		return stdout, stderr, errC
	}

	// Executes the session cleanup and processes any errors. Errors will be consolidated and published
	// to the error channel errC.
	// err - can be any error raised while executing the remote exec streaming request.
	completeStream := func(err error) {
		cleanupErr := cleanup()

		switch t := err.(type) {
		case *it.ExecError:
			t.Append(cleanupErr)
		case nil:
			err = cleanupErr
		default:
			merr := &multierror.Error{}
			merr = multierror.Append(merr, t, cleanupErr)
			err = merr.ErrorOrNil()
		}
		errC <- err
	}

	stdout, err = session.StdoutPipe()
	if err != nil {
		completeStream(err)
		return stdout, stderr, errC
	}

	stderr, err = session.StderrPipe()
	if err != nil {
		completeStream(err)
		return stdout, stderr, errC
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		completeStream(err)
		return stdout, stderr, errC
	}
	defer stdin.Close()

	err = session.Start(cmd.Cmd())
	if err != nil {
		completeStream(err)
		return stdout, stderr, errC
	}

	waitForCommandToFinish := func() {
		execErr := handleExecErr(session.Wait())
		completeStream(execErr)
	}

	go waitForCommandToFinish()

	return stdout, stderr, errC
}

// handleExecErr checks if the error is an xssh.ExitError and if so transforms it into a transport.ExecErr.
func handleExecErr(execErr error) error {
	if execErr != nil {
		var e *xssh.ExitError
		if errors.As(execErr, &e) {
			execErr = it.NewExecError(execErr, e.ExitStatus())
		}
	}

	return execErr
}

// Run runs the command and returns STDOUT, STDERR and the first error encountered.
func (t *transport) Run(ctx context.Context, cmd it.Command) (string, string, error) {
	var err error

	select {
	case <-ctx.Done():
		return "", "", ctx.Err()
	default:
	}

	stdout, stderr, errC := t.Stream(ctx, cmd)

	captureWait := sync.WaitGroup{}
	captureWait.Add(2)

	captureOutput := func(in io.Reader, out *bytes.Buffer) {
		// the stream reader can be nil, if the exec call fails early, so we need to guard against that
		if in != nil {
			scanner := bufio.NewScanner(in)
			scanner.Split(bufio.ScanBytes) // without this the scanner will swallow new lines

			for scanner.Scan() {
				out.WriteString(scanner.Text())
			}
		}

		captureWait.Done()
	}

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	go captureOutput(stdout, stdoutBuf)
	go captureOutput(stderr, stderrBuf)

	err = <-errC
	captureWait.Wait()

	// remove the trailing new line here, since in cases where the remote exec returns a single value
	// the trailing new line causes issues.
	sout := strings.TrimSuffix(stdoutBuf.String(), "\n")
	serr := strings.TrimSuffix(stderrBuf.String(), "\n")

	return sout, serr, handleExecErr(err)
}

// Close closes any underlying connections.
func (t *transport) Close() error {
	if t.client == nil {
		return nil
	}

	return t.client.Close()
}
