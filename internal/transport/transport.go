package transport

import (
	"context"
	"io"
)

// Transport is a generic transport interface
type Transport interface {
	Copy(context.Context, Copyable, string) error
	Run(context.Context, Command) (stdout string, stderr string, err error)
	Stream(context.Context, Command) (stdout io.Reader, stderr io.Reader, errC chan error)
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
