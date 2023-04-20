package ui

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

type UI interface {
	Stdout() io.Writer
	StdoutString() string
	Stderr() io.Writer
	StderrString() string
	CombinedOutput() string
	Write(stdout, stderr string) error
	Append(stdout, stderr string) error
}

type Output interface {
	io.ReadWriter
	String() string
}

func NewBuffered() *Buffered {
	b := &Buffered{
		stdoutBuf:   newBufferedWriter(),
		stderrBuf:   newBufferedWriter(),
		combinedBuf: newBufferedWriter(),
	}
	b.stdout = io.MultiWriter(b.stdoutBuf, b.combinedBuf)
	b.stderr = io.MultiWriter(b.stderrBuf, b.combinedBuf)

	return b
}

var _ UI = (*Buffered)(nil)

type Buffered struct {
	stdoutBuf   *bufferedWriter
	stderrBuf   *bufferedWriter
	combinedBuf *bufferedWriter
	stdout      io.Writer
	stderr      io.Writer
}

func (b *Buffered) Stdout() io.Writer {
	return b.stdout
}

func (b *Buffered) Stderr() io.Writer {
	return b.stderr
}

func (b *Buffered) StdoutString() string {
	return b.stdoutBuf.String()
}

func (b *Buffered) StderrString() string {
	return b.stderrBuf.String()
}

func (b *Buffered) CombinedOutput() string {
	return b.combinedBuf.String()
}

func (b *Buffered) Write(stdout, stderr string) error {
	var err error
	var err1 error

	if stdout != "" {
		_, err1 = b.stdout.Write([]byte(stdout))
		err = errors.Join(err, err1)
	}

	if stderr != "" {
		_, err1 = b.stderr.Write([]byte(stderr))
		err = errors.Join(err, err1)
	}

	return err
}

// Append is like Write but ensures that it's on a newline from any previously
// written data.
func (b *Buffered) Append(stdout, stderr string) error {
	var err error
	var err1 error

	if stdout != "" {
		_, err1 = b.stdout.Write([]byte(stdout))
		err = errors.Join(err, err1)
	}

	if stderr != "" {
		_, err1 = b.stderr.Write([]byte(stderr))
		err = errors.Join(err, err1)
	}

	return err
}

var (
	_ Output    = (*bufferedWriter)(nil)
	_ io.Writer = (*bufferedWriter)(nil)
)

func newBufferedWriter() *bufferedWriter {
	return &bufferedWriter{
		buf: &bytes.Buffer{},
		m:   sync.Mutex{},
	}
}

type bufferedWriter struct {
	buf *bytes.Buffer
	m   sync.Mutex
}

func (b *bufferedWriter) Read(p []byte) (int, error) {
	b.m.Lock()
	defer b.m.Unlock()

	unread := b.buf.Bytes()
	r := bytes.NewReader(unread)

	return r.Read(p)
}

func (b *bufferedWriter) Write(p []byte) (int, error) {
	b.m.Lock()
	defer b.m.Unlock()

	return b.buf.Write(p)
}

// Append is like write but adds a newline to the previous line
func (b *bufferedWriter) Append(p []byte) (int, error) {
	b.m.Lock()
	defer b.m.Unlock()

	if len(p) == 0 {
		return 0, nil
	}

	if b.buf.Len() > 0 {
		i, err := b.buf.Write([]byte("\n"))
		if err != nil {
			return i, err
		}
	}

	return b.buf.Write(p)
}

func (b *bufferedWriter) String() string {
	b.m.Lock()
	defer b.m.Unlock()

	unread := b.buf.Bytes()
	return string(unread)
}
