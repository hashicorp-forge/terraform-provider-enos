package ui

import (
	"bytes"
	"io"

	"github.com/hashicorp/go-multierror"
)

type UI interface {
	Stdout() Output
	Stderr() Output
	Write(stdout, stderr string) error
	Append(stdout, stderr string) error
}

type Output interface {
	io.ReadWriter
	String() string
}

func NewBuffered() *Buffered {
	return &Buffered{
		stdout: NewBufferedOut(),
		stderr: NewBufferedOut(),
	}
}

var _ UI = (*Buffered)(nil)

type Buffered struct {
	stdout *BufferedOut
	stderr *BufferedOut
}

func (b *Buffered) Stdout() Output {
	return b.stdout
}

func (b *Buffered) Stderr() Output {
	return b.stderr
}

func (b *Buffered) Write(stdout, stderr string) error {
	var err error
	merr := &multierror.Error{}

	if stdout != "" {
		_, err = b.stdout.Write([]byte(stdout))
		merr = multierror.Append(merr, err)
	}

	if stderr != "" {
		_, err = b.stderr.Write([]byte(stderr))
		merr = multierror.Append(merr, err)
	}

	return merr.ErrorOrNil()
}

// Append is like Write but ensures that it's on a newline from any previously
// written data.
func (b *Buffered) Append(stdout, stderr string) error {
	var err error
	merr := &multierror.Error{}

	if stdout != "" {
		_, err = b.stdout.Append([]byte(stdout))
		merr = multierror.Append(merr, err)
	}

	if stderr != "" {
		_, err = b.stderr.Append([]byte(stderr))
		merr = multierror.Append(merr, err)
	}

	return merr.ErrorOrNil()
}

func NewBufferedOut() *BufferedOut {
	return &BufferedOut{
		buf: &bytes.Buffer{},
	}
}

var _ Output = (*BufferedOut)(nil)

type BufferedOut struct {
	buf *bytes.Buffer
}

func (b *BufferedOut) Read(p []byte) (int, error) {
	unread := b.buf.Bytes()
	r := bytes.NewReader(unread)

	return r.Read(p)
}

func (b *BufferedOut) Write(p []byte) (int, error) {
	return b.buf.Write(p)
}

// Append is like write but adds a newline to the previous line
func (b *BufferedOut) Append(p []byte) (int, error) {
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

func (b *BufferedOut) String() string {
	unread := b.buf.Bytes()

	return string(unread)
}
