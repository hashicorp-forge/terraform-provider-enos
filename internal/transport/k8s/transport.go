// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package k8s

import (
	"context"
	"io"
	"path/filepath"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/kubernetes"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
)

// Transport is the kubernetes transport. It's public because sometimes callers need to access
// the raw client and data.
type Transport struct {
	Client    kubernetes.Client
	Namespace string
	Pod       string
	Container string
}

// TransportOpts are the options required in order to create the k8s transport.
type TransportOpts struct {
	KubeConfigBase64 string
	ContextName      string
	Namespace        string
	Pod              string
	Container        string
}

var _ it.Transport = (*Transport)(nil)

// NewTransport takes transport opts and return a new instance of the kubernetes transport.
func NewTransport(opts TransportOpts) (it.Transport, error) {
	client, err := kubernetes.NewClient(kubernetes.ClientCfg{
		KubeConfigBase64: opts.KubeConfigBase64,
		ContextName:      opts.ContextName,
	})
	if err != nil {
		return nil, err
	}

	namespace := "default"
	if opts.Namespace != "" {
		namespace = opts.Namespace
	}

	transport := &Transport{
		Client:    client,
		Namespace: namespace,
		Pod:       opts.Pod,
		Container: opts.Container,
	}

	return transport, nil
}

func (t Transport) Type() it.TransportType {
	return it.TransportType("kubernetes")
}

// Copy copies the copyable src to the dst on a Pod as specified in the transport options.
func (t Transport) Copy(ctx context.Context, src it.Copyable, dst string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	request := t.Client.NewExecRequest(kubernetes.ExecRequestOpts{
		Command:   "tar -xmf - -C " + filepath.Dir(dst),
		StdIn:     true,
		Namespace: t.Namespace,
		Pod:       t.Pod,
		Container: t.Container,
	})

	return it.Copy(ctx, it.TarCopyWriter(src, dst, request.Streams().StdinWriter()), dst, request)
}

// Run runs the provided command on a remote Pod as specified th in the transport config. Run blocks
// until the command execution has completed.
func (t Transport) Run(ctx context.Context, cmd it.Command) (stdout, stderr string, err error) {
	return it.Run(ctx, t.Client.NewExecRequest(kubernetes.ExecRequestOpts{
		Command:   cmd.Cmd(),
		Namespace: t.Namespace,
		Pod:       t.Pod,
		Container: t.Container,
	}))
}

// Stream runs the provided command on a remote Pod and streams the results. Stream does not block and
// is done when the error channel has either an error or nil.
func (t Transport) Stream(ctx context.Context, command it.Command) (stdout, stderr io.Reader, errC chan error) {
	return it.Stream(ctx, t.Client.NewExecRequest(kubernetes.ExecRequestOpts{
		Command:   command.Cmd(),
		Namespace: t.Namespace,
		Pod:       t.Pod,
		Container: t.Container,
	}))
}

func (t Transport) Close() error {
	// nothing to do for the k8s transport.
	return nil
}
