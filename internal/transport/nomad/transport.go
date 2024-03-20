// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"context"
	"io"
	"path/filepath"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/nomad"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
)

// transport a Nomad based transport implementation.
type transport struct {
	client       nomad.Client
	allocationID string
	taskName     string
}

// TransportOpts are the options required in order to create the nomad transport.
type TransportOpts struct {
	Host         string
	SecretID     string
	AllocationID string
	TaskName     string
}

var _ it.Transport = (*transport)(nil)

func NewTransport(opts TransportOpts) (it.Transport, error) {
	client, err := nomad.NewClient(nomad.ClientCfg{
		Host:     opts.Host,
		SecretID: opts.SecretID,
	})
	if err != nil {
		return nil, err
	}

	return &transport{
		client:       client,
		allocationID: opts.AllocationID,
		taskName:     opts.TaskName,
	}, nil
}

func (t *transport) Type() it.TransportType {
	return it.TransportType("nomad")
}

func (t *transport) Copy(ctx context.Context, src it.Copyable, dst string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	request := t.client.NewExecRequest(nomad.ExecRequestOpts{
		Command:      []string{"tar", "-xmf", "-", "-C", filepath.Dir(dst)},
		StdIn:        true,
		AllocationID: t.allocationID,
		TaskName:     t.taskName,
	})

	return it.Copy(ctx, it.TarCopyWriter(src, dst, request.Streams().StdinWriter()), dst, request)
}

func (t *transport) Run(ctx context.Context, command it.Command) (stdout string, stderr string, err error) {
	return it.Run(ctx, t.client.NewExecRequest(nomad.ExecRequestOpts{
		Command:      []string{"sh", "-c", command.Cmd()},
		AllocationID: t.allocationID,
		TaskName:     t.taskName,
	}))
}

func (t *transport) Stream(ctx context.Context, command it.Command) (stdout io.Reader, stderr io.Reader, errC chan error) {
	return it.Stream(ctx, t.client.NewExecRequest(nomad.ExecRequestOpts{
		AllocationID: t.allocationID,
		Command:      []string{"sh", "-c", command.Cmd()},
		TaskName:     t.taskName,
	}))
}

func (t *transport) Close() error {
	t.client.Close()

	return nil
}
