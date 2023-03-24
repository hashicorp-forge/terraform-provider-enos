package kubernetes

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
)

const (
	kubeConfigEnvVar = "KUBECONFIG"
	defaultNamespace = "default"
)

// GetPodInfoRequest a request for getting pod info
type GetPodInfoRequest struct {
	// Namespace the namespace that the pod is in
	Namespace string
	// Name the name of the pod
	Name string
}

// Client A wrapper around the k8s clientset that provides an api that the provider can use
type Client interface {
	// NewExecRequest creates a new exec request for the given opts
	NewExecRequest(opts ExecRequestOpts) it.ExecRequest
	// QueryPodInfos gets a slice of all the pods that match the QueryPodInfosRequest. If the namespace
	// is not provided in the request it will be defaulted to 'default'.
	QueryPodInfos(ctx context.Context, req QueryPodInfosRequest) ([]PodInfo, error)
	// GetPodInfo get pod info for the pod that matches the request. If the namespace is not provided
	// in the request it will be defaulted to 'default'.
	GetPodInfo(ctx context.Context, req GetPodInfoRequest) (*PodInfo, error)
	// GetLogs gets the logs for the provided QueryPodInfosRequest
	GetLogs(ctx context.Context, req GetPodLogsRequest) (*GetPodLogsResponse, error)
}

type ClientCfg struct {
	KubeConfigBase64 string
	ContextName      string
}

// NewClient creates a new Kubernetes Client.
func NewClient(cfg ClientCfg) (Client, error) {
	clientset, restConfig, err := createClientset(cfg.KubeConfigBase64, cfg.ContextName)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset, due to: %w", err)
	}

	return &client{
		clientset:  clientset,
		restConfig: restConfig,
	}, nil
}

// execRequest A kubernetes based exec request
type execRequest struct {
	client  *client
	opts    ExecRequestOpts
	streams *it.ExecStreams
}

type ExecRequestOpts struct {
	Command   string
	StdIn     bool
	Namespace string
	Pod       string
	Container string
}

type QueryPodInfosRequest struct {
	Namespace     string
	LabelSelector string
	FieldSelector string
}

type PodInfo struct {
	Name       string
	Namespace  string
	Containers []string
}

type GetPodLogsRequest struct {
	ContextName string
	Namespace   string
	Pod         string
	Container   string
}

type GetPodLogsResponse struct {
	ContextName string
	Namespace   string
	Pod         string
	Container   string
	Logs        []byte
}

var _ remoteflight.GetLogsResponse = (*GetPodLogsResponse)(nil)

// GetAppName implements remoteflight.GetLogsResponse.GetAppName
func (p GetPodLogsResponse) GetAppName() string {
	return p.Container
}

func (p GetPodLogsResponse) GetLogFileName() string {
	parts := []string{p.ContextName, p.Namespace, p.Pod, p.Container}

	filename := strings.Join(parts, "_")
	filename = fmt.Sprintf("%s.log", filename)

	return filename
}

func (p GetPodLogsResponse) GetLogs() []byte {
	return p.Logs
}

func (e *execRequest) Streams() *it.ExecStreams {
	return e.streams
}

// Exec executes a command on a remote pod as would be done via `kubectl exec`.
func (e *execRequest) Exec(ctx context.Context) *it.ExecResponse {
	response := it.NewExecResponse()

	select {
	case <-ctx.Done():
		response.ExecErr <- ctx.Err()
		return response
	default:
	}

	executor, err := e.client.createExecutor(*e)
	if err != nil {
		response.ExecErr <- err
		return response
	}

	streams := e.streams
	response.Stdout = streams.Stdout()
	response.Stderr = streams.Stderr()

	stream := func(stdout, stderr io.Writer) {
		defer streams.Close()
		execErr := executor.Stream(remotecommand.StreamOptions{
			Stdout: stdout,
			Stderr: stderr,
			Stdin:  streams.Stdin(),
		})
		if execErr != nil {
			var e exec.CodeExitError
			if errors.As(execErr, &e) {
				execErr = it.NewExecError(execErr, e.ExitStatus())
			}
		}
		response.ExecErr <- execErr
	}

	go stream(streams.StdoutWriter(), streams.StderrWriter())

	return response
}

type client struct {
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
}

func (c *client) NewExecRequest(opts ExecRequestOpts) it.ExecRequest {
	return &execRequest{
		client:  c,
		opts:    opts,
		streams: it.NewExecStreams(opts.StdIn),
	}
}

// QueryPodInfos queries Kubernetes using search criteria for the given QueryPodInfosRequest and returns a
// list of pod infos that match the query.
func (c *client) QueryPodInfos(ctx context.Context, req QueryPodInfosRequest) ([]PodInfo, error) {
	namespace := req.Namespace
	if strings.TrimSpace(namespace) == "" {
		namespace = defaultNamespace
	}

	podList, err := c.clientset.
		CoreV1().
		Pods(namespace).
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
			Name:       pod.Name,
			Namespace:  pod.Namespace,
			Containers: getContainers(pod),
		})
	}

	return pods, err
}

func (c *client) GetPodInfo(ctx context.Context, req GetPodInfoRequest) (*PodInfo, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("cannot get pod info without the pod name")
	}

	namespace := req.Namespace
	if strings.TrimSpace(namespace) == "" {
		namespace = defaultNamespace
	}

	pod, err := c.clientset.
		CoreV1().
		Pods(namespace).
		Get(ctx, req.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get a pod for name: %s and namespace: %s, due to: %w", req.Name, req.Namespace, err)
	}

	return &PodInfo{
		Name:       req.Name,
		Namespace:  namespace,
		Containers: getContainers(*pod),
	}, nil
}

func (c *client) GetLogs(ctx context.Context, req GetPodLogsRequest) (*GetPodLogsResponse, error) {
	if strings.TrimSpace(req.Pod) == "" {
		return nil, fmt.Errorf("cannot get pod logs without providing a pod name")
	}

	namespace := req.Namespace
	if strings.TrimSpace(namespace) == "" {
		namespace = defaultNamespace
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("failed to get logs, due to: %w", ctx.Err())
	default:
		// if the context is not done, just carry on...
	}

	getLogsReq := c.clientset.CoreV1().
		Pods(namespace).
		GetLogs(req.Pod, &v1.PodLogOptions{
			Container: req.Container,
		})

	podLogs, err := getLogsReq.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod logs, for Pod: [%s], in Namepsace: [%s], due to: %w", req.Pod, req.Namespace, err)
	}
	defer podLogs.Close()

	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return nil, fmt.Errorf("failed to copy pod logs, for Pod: [%s], in Namepsace: [%s], due to: %w", req.Pod, req.Namespace, err)
	}

	return &GetPodLogsResponse{
		ContextName: req.ContextName,
		Namespace:   namespace,
		Pod:         req.Pod,
		Container:   req.Container,
		Logs:        buf.Bytes(),
	}, nil
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

func (c *client) createExecutor(execRequest execRequest) (remotecommand.Executor, error) {
	request := c.clientset.CoreV1().RESTClient().
		Post().
		Namespace(execRequest.opts.Namespace).
		Resource("pods").
		Name(execRequest.opts.Pod).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Command:   []string{"/bin/sh", "-c", execRequest.opts.Command},
			Stdin:     execRequest.Streams().Stdin() != nil,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
			Container: execRequest.opts.Container,
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

func getContainers(pod v1.Pod) []string {
	var containers []string
	for _, container := range pod.Spec.Containers {
		containers = append(containers, container.Name)
	}
	return containers
}
