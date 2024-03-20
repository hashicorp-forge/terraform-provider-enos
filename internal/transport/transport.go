// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package transport

import (
	"context"
	"io"
)

// Transport is a generic transport interface.
type Transport interface {
	Copy(ctx context.Context, body Copyable, dest string) error
	Run(ctx context.Context, cmd Command) (stdout, stderr string, err error)
	Stream(ctx context.Context, cmd Command) (stdout, stderr io.Reader, errC chan error)
	Type() TransportType
	io.Closer
}

// Command represents a command to run.
type Command interface {
	Cmd() string
}

// Copyable is an interface for a copyable file.
type Copyable interface {
	io.ReadSeekCloser
	Size() int64
}

// TransportType is the string representation of the transport type.
type TransportType string
