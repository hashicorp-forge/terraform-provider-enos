package k8s

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"path/filepath"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/go-multierror"
)

type transport struct {
	client    *client
	namespace string
	pod       string
	container string
}

// TransportOpts are the options required in order to create the k8s transport
type TransportOpts struct {
	KubeConfigPath string
	ContextName    string
	Namespace      string
	Pod            string
	Container      string
}

var _ it.Transport = (*transport)(nil)

func NewTransport(opts TransportOpts) (it.Transport, error) {
	client, err := NewClient(clientCfg{
		kubeConfigPath: opts.KubeConfigPath,
		contextName:    opts.ContextName,
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

	stdInReader, stdInWriter := io.Pipe()

	writeErr := make(chan error, 1)

	writeInput := func() {
		defer stdInWriter.Close()

		writer := tar.NewWriter(stdInWriter)

		err := writer.WriteHeader(&tar.Header{
			Name: filepath.Base(dst),
			Mode: 0o644,
			Size: src.Size(),
		})
		if err != nil {
			writeErr <- err
			return
		}

		_, err = io.Copy(writer, src)
		defer writer.Close()
		writeErr <- err
	}

	go writeInput()

	response := t.client.Exec(ctx, execRequest{
		command:   fmt.Sprintf("tar -xmf - -C %s", filepath.Dir(dst)),
		stdIn:     stdInReader,
		namespace: t.namespace,
		pod:       t.pod,
		container: t.container,
	})

	_, stderr, execErr := response.WaitForResults()

	merr := &multierror.Error{}

	if err := <-writeErr; err != nil {
		merr = multierror.Append(merr, err)
	}

	if execErr != nil {
		merr = multierror.Append(merr, execErr)
	}

	if len(stderr) > 0 {
		merr = multierror.Append(merr, fmt.Errorf("failed to copy to dst: [%s], due to: [%s]", dst, stderr))
	}

	return merr.ErrorOrNil()
}

// Run runs the provided command on a remote Pod as specified th in the transport config. Run blocks
// until the command execution has completed.
func (t transport) Run(ctx context.Context, cmd it.Command) (stdout string, stderr string, err error) {
	response := t.client.Exec(ctx, execRequest{
		command:   cmd.Cmd(),
		namespace: t.namespace,
		pod:       t.pod,
		container: t.container,
	})

	return response.WaitForResults()
}

// Stream runs the provided command on a remote Pod and streams the results. Stream does not block and
// is done when the error channel has either an error or nil.
func (t transport) Stream(ctx context.Context, command it.Command) (stdout io.Reader, stderr io.Reader, errC chan error) {
	response := t.client.Exec(ctx, execRequest{
		command:   command.Cmd(),
		namespace: t.namespace,
		pod:       t.pod,
		container: t.container,
	})
	stdout = response.stdout
	stderr = response.stderr
	errC = response.execErr

	return stdout, stderr, errC
}

func (t transport) Close() error {
	// nothing to do for the k8s transport.
	return nil
}
