package transport

import (
	"context"
	"io"
	"os"
)

// Transport is a generic transport interface
type Transport interface {
	Copy(context.Context, Copyable, string) error
	Run(context.Context, string) error
	Stream(context.Context, string) (stdout io.Reader, stderr io.Reader, err error)
}

// Copyable is an interface for a copyable file
type Copyable interface {
	io.ReadCloser
	io.Seeker
	os.FileInfo
}
