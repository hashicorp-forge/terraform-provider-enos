package kubernetes

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"

	it "github.com/hashicorp/enos-provider/internal/transport"

	"github.com/hashicorp/go-multierror"
)

const kubeConfigEnvVar = "KUBECONFIG"

type Client struct {
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
}

type ClientCfg struct {
	KubeConfigBase64 string
	ContextName      string
}

type ExecRequest struct {
	Command   string
	StdIn     io.Reader
	Namespace string
	Pod       string
	Container string
}

type ExecResponse struct {
	Stdout  io.Reader
	Stderr  io.Reader
	ExecErr chan error
}

type GetPodInfoRequest struct {
	Namespace     string
	LabelSelector string
	FieldSelector string
}

type PodInfo struct {
	Name      string
	Namespace string
}

// WaitForResults waits for the execution to finish and returns the stdout, stderr and the execution error
// if there is any.
func (e *ExecResponse) WaitForResults() (stdout, stderr string, err error) {
	wg := sync.WaitGroup{}
	wg.Add(2)

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
	go captureOutput(stdoutBuf, e.Stdout, streamWriteErrC, "stdout")
	go captureOutput(stderrBuf, e.Stderr, streamWriteErrC, "stderr")

	execErr := <-e.ExecErr

	wg.Wait()
	close(streamWriteErrC)

	writeErrs := &multierror.Error{}
	for streamErr := range streamWriteErrC {
		writeErrs = multierror.Append(writeErrs, streamErr)
	}

	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if execErr != nil {
		switch e := execErr.(type) {
		case *it.ExecError:
			e.Append(writeErrs)
			err = e
		default:
			// in the case that the exec error is not a transport.ExecError we create a new multierror
			// here and append the exec error first then the other errors.
			allErrors := &multierror.Error{}
			allErrors = multierror.Append(allErrors, execErr, writeErrs)
			err = allErrors.ErrorOrNil()
		}
	} else {
		err = writeErrs.ErrorOrNil()
	}

	return stdout, stderr, err
}

func NewClient(cfg ClientCfg) (*Client, error) {
	clientset, restConfig, err := createClientset(cfg.KubeConfigBase64, cfg.ContextName)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset, due to: %w", err)
	}

	return &Client{
		clientset:  clientset,
		restConfig: restConfig,
	}, nil
}

// Exec executes a command on a remote pod as would be done via `kubectl exec`.
func (c *Client) Exec(ctx context.Context, request ExecRequest) *ExecResponse {
	response := &ExecResponse{
		ExecErr: make(chan error, 1),
	}

	select {
	case <-ctx.Done():
		response.ExecErr <- ctx.Err()
		return response
	default:
	}

	executor, err := c.createExecutor(request)
	if err != nil {
		response.ExecErr <- err
		return response
	}

	stdout, stdOutWriter := io.Pipe()
	stderr, stdErrWriter := io.Pipe()
	response.Stdout = stdout
	response.Stderr = stderr

	completeExec := func() {
		stdOutWriter.Close()
		stdErrWriter.Close()
	}

	stream := func(stdout, stderr io.Writer) {
		defer completeExec()
		execErr := executor.Stream(remotecommand.StreamOptions{
			Stdout: stdout,
			Stderr: stderr,
			Stdin:  request.StdIn,
		})
		if execErr != nil {
			var e exec.CodeExitError
			if errors.As(execErr, &e) {
				execErr = it.NewExecError(execErr, e.ExitStatus())
			}
		}
		response.ExecErr <- execErr
	}

	go stream(stdOutWriter, stdErrWriter)

	return response
}

// GetPodInfos queries Kubernetes using search criteria for the given GetPodInfoRequest and returns a
// list of pod infos that match the query.
func (c *Client) GetPodInfos(ctx context.Context, req GetPodInfoRequest) ([]PodInfo, error) {
	podList, err := c.clientset.
		CoreV1().
		Pods(req.Namespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: req.LabelSelector,
			FieldSelector: req.FieldSelector,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to get pods names, due to: %w", err)
	}

	var pods []PodInfo
	for _, pod := range podList.Items {
		pods = append(pods, PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		})
	}

	return pods, err
}

// DecodeAndLoadKubeConfig decodes a base64 encoded kubeconfig and attempts to load the Config.
func DecodeAndLoadKubeConfig(encodedKubeConfig string) (*clientcmdapi.Config, error) {
	decodedKubeConfig, err := base64.StdEncoding.DecodeString(encodedKubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode kubeconfig, due to: %w", err)
	}

	kubeConfig, err := clientcmd.Load(decodedKubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig due to: %w", err)
	}

	return kubeConfig, nil
}

// createClientset creates the clientset and rest config for the provided kubeconfig and context name.
// The kubeconfig must be base64 encoded.
func createClientset(kubeConfigBase64, contextName string) (*kubernetes.Clientset, *rest.Config, error) {
	kubeConfig, err := DecodeAndLoadKubeConfig(kubeConfigBase64)
	if err != nil {
		return nil, nil, err
	}

	config, err := clientcmd.NewNonInteractiveClientConfig(*kubeConfig, contextName, nil, clientcmd.NewDefaultClientConfigLoadingRules()).ClientConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create rest client config for context [%s], due to: %w", contextName, err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create kuberentes clientset for context: [%s] due to: %w", contextName, err)
	}

	return clientset, config, nil
}

func (c *Client) createExecutor(execRequest ExecRequest) (remotecommand.Executor, error) {
	request := c.clientset.CoreV1().RESTClient().
		Post().
		Namespace(execRequest.Namespace).
		Resource("pods").
		Name(execRequest.Pod).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Command:   []string{"/bin/sh", "-c", execRequest.Command},
			Stdin:     execRequest.StdIn != nil,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
			Container: execRequest.Container,
		}, scheme.ParameterCodec)

	return remotecommand.NewSPDYExecutor(c.restConfig, "POST", request.URL())
}

// GetKubeConfigPath given a kubeConfigPath that might be empty gets a kubeconfig path, by returning
// the provided value if is not empty, or the value of the kubeConfigEnv if set, or the default
// kubeconfig path in the users' home dir (~/.kube/config)
func GetKubeConfigPath(kubeConfigPath string) (string, error) {
	if kubeConfigPath != "" {
		return kubeConfigPath, nil
	}

	kubeConfigEnv, ok := os.LookupEnv(kubeConfigEnvVar)
	if ok {
		list := filepath.SplitList(kubeConfigEnv)
		length := len(list)

		switch {
		case length == 0:
			return list[0], nil
		case length > 1:
			return "", fmt.Errorf("ambiguous kubeconfig path, using 'KUBECONFIG' env var value: [%s]", kubeConfigEnv)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home dir, when looking for the kubeconfig, due to: %w", err)
	}

	return filepath.Join(homeDir, ".kube", "config"), nil
}
