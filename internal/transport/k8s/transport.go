package k8s

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/hashicorp/enos-provider/internal/kubernetes"
	it "github.com/hashicorp/enos-provider/internal/transport"
)

type transport struct {
	client    kubernetes.Client
	namespace string
	pod       string
	container string
}

// TransportOpts are the options required in order to create the k8s transport.
type TransportOpts struct {
	KubeConfigBase64 string
	ContextName      string
	Namespace        string
	Pod              string
	Container        string
}

var _ it.Transport = (*transport)(nil)

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

	transport := &transport{
		client:    client,
		namespace: namespace,
		pod:       opts.Pod,
		container: opts.Container,
	}

	return transport, nil
}

// Copy copies the copyable src to the dst on a Pod as specified in the transport options.
func (t transport) Copy(ctx context.Context, src it.Copyable, dst string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	request := t.client.NewExecRequest(kubernetes.ExecRequestOpts{
		Command:   fmt.Sprintf("tar -xmf - -C %s", filepath.Dir(dst)),
		StdIn:     true,
		Namespace: t.namespace,
		Pod:       t.pod,
		Container: t.container,
	})

	return it.Copy(ctx, it.TarCopyWriter(src, dst, request.Streams().StdinWriter()), dst, request)
}

// Run runs the provided command on a remote Pod as specified th in the transport config. Run blocks
// until the command execution has completed.
func (t transport) Run(ctx context.Context, cmd it.Command) (stdout, stderr string, err error) {
	return it.Run(ctx, t.client.NewExecRequest(kubernetes.ExecRequestOpts{
		Command:   cmd.Cmd(),
		Namespace: t.namespace,
		Pod:       t.pod,
		Container: t.container,
	}))
}

// Stream runs the provided command on a remote Pod and streams the results. Stream does not block and
// is done when the error channel has either an error or nil.
func (t transport) Stream(ctx context.Context, command it.Command) (stdout, stderr io.Reader, errC chan error) {
	return it.Stream(ctx, t.client.NewExecRequest(kubernetes.ExecRequestOpts{
		Command:   command.Cmd(),
		Namespace: t.namespace,
		Pod:       t.pod,
		Container: t.container,
	}))
}

func (t transport) Close() error {
	// nothing to do for the k8s transport.
	return nil
}
