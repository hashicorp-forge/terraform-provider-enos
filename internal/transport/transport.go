package transport

import (
	"context"
	"io"

	"github.com/hashicorp/go-multierror"
)

// Transport is a generic transport interface
type Transport interface {
	Copy(context.Context, Copyable, string) error
	Run(context.Context, Command) (stdout, stderr string, err error)
	Stream(context.Context, Command) (stdout, stderr io.Reader, errC chan error)
	io.Closer
}

// Command represents a command to run
type Command interface {
	Cmd() string
}

// Copyable is an interface for a copyable file
type Copyable interface {
	io.ReadSeekCloser
	Size() int64
}

// ExecError An exec error is a wrapper error that all transport implementations should return if the
// exec failed with an exit code
type ExecError struct {
	merr     *multierror.Error
	exitCode int
}

// NewExecError creates a new ExecError with the provided exitCode and wrapped error
func NewExecError(err error, exitCode int) error {
	merr := &multierror.Error{}
	merr = multierror.Append(merr, err)
	return &ExecError{
		merr:     merr,
		exitCode: exitCode,
	}
}

// Error returns the wrapped error
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
