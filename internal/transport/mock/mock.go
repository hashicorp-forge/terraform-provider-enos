// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package mock

import (
	"bufio"
	"context"
	"io"

	it "github.com/hashicorp/enos-provider/internal/transport"
)

type mockTransport struct{}

// New creates a new Mock Transport client, that does nothing, useful for testing.
func New() it.Transport {
	return &mockTransport{}
}

func (m *mockTransport) Type() it.TransportType {
	return it.TransportType("mock")
}

func (m *mockTransport) Copy(ctx context.Context, copyable it.Copyable, s string) error {
	return nil
}

func (m *mockTransport) Run(ctx context.Context, command it.Command) (stdout, stderr string, err error) {
	return "", "", nil
}

func (m *mockTransport) Stream(ctx context.Context, command it.Command) (stdout, stderr io.Reader, errC chan error) {
	return &bufio.Reader{}, &bufio.Reader{}, nil
}

func (m *mockTransport) Close() error {
	return nil
}
