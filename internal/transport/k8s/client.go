package k8s

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/hashicorp/go-multierror"
)

type client struct {
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
}

type clientCfg struct {
	kubeConfigPath string
	contextName    string
}

type execRequest struct {
	command   string
	stdIn     io.Reader
	namespace string
	pod       string
	container string
}

type execResponse struct {
	stdout  io.Reader
	stderr  io.Reader
	execErr chan error
}

// WaitForResults waits for the execution to finish and returns the stdout, stderr and the execution error
// if there is any.
func (e *execResponse) WaitForResults() (stdout string, stderr string, err error) {
	wg := sync.WaitGroup{}
	wg.Add(2)

	err = &multierror.Error{}

	captureOutput := func(writer io.StringWriter, reader io.Reader, errC chan error, stream string) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			_, execErr := writer.WriteString(scanner.Text())
			if execErr != nil {
				errC <- fmt.Errorf("failed to write exec %s, due to %v", stream, execErr)
				break
			}
		}
		wg.Done()
	}

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	streamWriteErrC := make(chan error, 2)
	go captureOutput(stdoutBuf, e.stdout, streamWriteErrC, "stdout")
	go captureOutput(stderrBuf, e.stderr, streamWriteErrC, "stderr")

	execErr := <-e.execErr
	if execErr != nil {
		err = multierror.Append(err, execErr)
	}

	wg.Wait()
	close(streamWriteErrC)
	for streamErr := range streamWriteErrC {
		err = multierror.Append(err, streamErr)
	}

	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	return stdout, stderr, err
}

func NewClient(cfg clientCfg) (*client, error) {
	clientset, restConfig, err := createClientset(cfg.kubeConfigPath, cfg.contextName)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset, due to: %w", err)
	}

	return &client{
		clientset:  clientset,
		restConfig: restConfig,
	}, nil
}

// Exec executes a command on a remote pod as would be done via `kubectl exec`.
func (c *client) Exec(ctx context.Context, request execRequest) *execResponse {
	response := &execResponse{
		execErr: make(chan error, 1),
	}

	select {
	case <-ctx.Done():
		response.execErr <- ctx.Err()
		return response
	default:
	}

	executor, err := c.createExecutor(request)
	if err != nil {
		response.execErr <- err
		return response
	}

	stdout, stdOutWriter := io.Pipe()
	stderr, stdErrWriter := io.Pipe()
	response.stdout = stdout
	response.stderr = stderr

	completeExec := func() {
		stdOutWriter.Close()
		stdErrWriter.Close()
	}

	stream := func(stdout, stderr io.Writer) {
		defer completeExec()
		response.execErr <- executor.Stream(remotecommand.StreamOptions{
			Stdout: stdout,
			Stderr: stderr,
			Stdin:  request.stdIn,
		})
	}

	go stream(stdOutWriter, stdErrWriter)

	return response
}

func createClientset(kubeConfigPath, contextName string) (*kubernetes.Clientset, *rest.Config, error) {
	kubeConfig, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load kubeconfig [%s] for context [%s], due to: %w", kubeConfigPath, contextName, err)
	}

	config, err := clientcmd.NewNonInteractiveClientConfig(*kubeConfig, contextName, nil, clientcmd.NewDefaultClientConfigLoadingRules()).ClientConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create rest client config using kubeconfig [%s] and context [%s], due to: %w", kubeConfigPath, contextName, err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kuberentes clientset using kubeconfig [%s] and context: [%s] due to: %w", kubeConfigPath, contextName, err)
	}

	return clientset, config, nil
}

func (c *client) createExecutor(execRequest execRequest) (remotecommand.Executor, error) {
	request := c.clientset.CoreV1().RESTClient().
		Post().
		Namespace(execRequest.namespace).
		Resource("pods").
		Name(execRequest.pod).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Command:   []string{"/bin/sh", "-c", execRequest.command},
			Stdin:     execRequest.stdIn != nil,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
			Container: execRequest.container,
		}, scheme.ParameterCodec)

	return remotecommand.NewSPDYExecutor(c.restConfig, "POST", request.URL())
}
