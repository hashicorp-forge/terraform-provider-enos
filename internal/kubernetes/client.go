// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	cv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/exec"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/retry"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight"
	istrings "github.com/hashicorp-forge/terraform-provider-enos/internal/strings"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
)

const (
	kubeConfigEnvVar = "KUBECONFIG"
	defaultNamespace = "default"
)

// GetPodInfoRequest a request for getting pod info.
type GetPodInfoRequest struct {
	// Namespace the namespace that the pod is in
	Namespace string
	// Name the name of the pod
	Name string
}

// Client A wrapper around the k8s clientset that provides an api that the provider can use.
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
	// ListPods returns all pods matching the given request
	ListPods(ctx context.Context, req *ListPodsRequest) (*ListPodsResponse, error)
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

// execRequest A kubernetes based exec request.
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
	// ExpectedPodCount the expected number of pods that should be returned from the query
	ExpectedPodCount int
	// WaitTimeout the amount of time to wait for the pods to be in the 'RUNNING' state
	WaitTimeout time.Duration
}

type PodInfo struct {
	Name       string
	Namespace  string
	Containers []string
	Pod        *v1.Pod
}

type GetPodLogsRequest struct {
	ContextName string
	Namespace   string
	Pod         string
	Container   string
}

var _ remoteflight.GetLogsResponse = (*GetPodLogsResponse)(nil)

type GetPodLogsResponse struct {
	ContextName string
	Namespace   string
	Pod         string
	Container   string
	Logs        []byte
}

// ListPodsRequest is a request to list the pods matching the search criteria.
type ListPodsRequest struct {
	// The kubernetes namespace to query
	Namespace string
	// Label selectors to filter by
	LabelSelectors []string
	// Field selectors to filter by
	FieldSelectors []string
	// How we'll retry the request
	*retry.Retrier
	// Options we'll apply to the retrier
	RetryOpts []retry.RetrierOpt
}

// ListPodsResponse is ListPods response.
type ListPodsResponse struct {
	Pods *Pods
}

// Pods is an alias for a v1.PodList so we can attach methods to it.
type Pods v1.PodList

// ListPodsRequestOpt is a functional option for NewListPodsRequest.
type ListPodsRequestOpt func(*ListPodsRequest)

// NewListPodsRequest takes NewListPodsRequestOpt's and returns a new instance of ListPodsRequest.
func NewListPodsRequest(opts ...ListPodsRequestOpt) *ListPodsRequest {
	req := &ListPodsRequest{
		Namespace: defaultNamespace,
		Retrier: &retry.Retrier{
			MaxRetries:     retry.MaxRetriesUnlimited,
			RetryInterval:  retry.IntervalExponential(2 * time.Second),
			OnlyRetryError: []error{},
		},
	}

	for _, opt := range opts {
		opt(req)
	}

	for _, opt := range req.RetryOpts {
		opt(req.Retrier)
	}

	return req
}

// WithListPodsRequestNamespace allows the caller to define namespace for the ListPods request.
func WithListPodsRequestNamespace(name string) ListPodsRequestOpt {
	return func(req *ListPodsRequest) {
		req.Namespace = name
	}
}

// WithListPodsRequestFieldSelectors allows the caller to define field selectors for the ListPods
// request.
func WithListPodsRequestFieldSelectors(selectors []string) ListPodsRequestOpt {
	return func(req *ListPodsRequest) {
		req.FieldSelectors = selectors
	}
}

// WithListPodsRequestLabelSelectors allows the caller to define label selectors for the ListPods
// request.
func WithListPodsRequestLabelSelectors(selectors []string) ListPodsRequestOpt {
	return func(req *ListPodsRequest) {
		req.LabelSelectors = selectors
	}
}

// WithListPodsRequestRetryOpts allows the caller to define retry options for the ListPods request.
func WithListPodsRequestRetryOpts(opts ...retry.RetrierOpt) ListPodsRequestOpt {
	return func(req *ListPodsRequest) {
		req.RetryOpts = opts
	}
}

// GetAppName implements remoteflight.GetLogsResponse.GetAppName.
func (p GetPodLogsResponse) GetAppName() string {
	return p.Container
}

func (p GetPodLogsResponse) GetLogFileName() string {
	parts := []string{p.ContextName, p.Namespace, p.Pod, p.Container}

	filename := strings.Join(parts, "_")
	filename = filename + ".log"

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
		execErr := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
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

	queryFunc := func(queryCtx context.Context) (interface{}, error) {
		podList, err := c.clientset.
			CoreV1().
			Pods(namespace).
			List(ctx, metav1.ListOptions{
				LabelSelector: req.LabelSelector,
				FieldSelector: req.FieldSelector,
			})
		if err != nil {
			return nil, fmt.Errorf("failed to query pods for request: %#v, due to: %w", req, err)
		}
		if podList == nil || len(podList.Items) == 0 {
			return nil, fmt.Errorf("failed to find any pods for request: %#v", req)
		}

		if req.ExpectedPodCount > 0 {
			if len(podList.Items) != req.ExpectedPodCount {
				return nil, fmt.Errorf("expected to find: [%d] pods found: [%d]", req.ExpectedPodCount, len(podList.Items))
			}
		}

		for _, pod := range podList.Items {
			if pod.Status.Phase != v1.PodRunning {
				return nil, fmt.Errorf("expected pod 'Phase' to be [%s], was: [%s], for pod: [%s]",
					v1.PodRunning, pod.Status.Phase, pod.Name)
			}
		}

		return podList.Items, nil
	}

	var err error
	var result interface{}
	if req.WaitTimeout > 0 {
		retrier, err := retry.NewRetrier(
			retry.WithRetrierFunc(queryFunc),
			retry.WithIntervalFunc(retry.IntervalFibonacci(time.Second)),
			retry.WithMaxRetries(10), // we will rely on the timeout context below to end the retires
		)
		if err != nil {
			return nil, fmt.Errorf("failed to query pods, due to: %w", err)
		}

		queryCtx, cancel := context.WithTimeout(ctx, req.WaitTimeout)
		defer cancel()

		result, err = retry.Retry(queryCtx, retrier)
		if err != nil {
			return nil, fmt.Errorf("failed to query pods, due to: %w", err)
		}
	} else {
		result, err = queryFunc(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query pods, due to: %w", err)
		}
	}

	pods, ok := result.([]v1.Pod)
	if !ok {
		return nil, errors.New("failed to process pod query result")
	}

	podInfos := make([]PodInfo, len(pods))
	for i := range pods {
		podInfos[i] = PodInfo{
			Name:       pods[i].Name,
			Namespace:  pods[i].Namespace,
			Containers: getContainers(pods[i]),
			Pod:        &pods[i],
		}
	}

	return podInfos, err
}

func (c *client) GetPodInfo(ctx context.Context, req GetPodInfoRequest) (*PodInfo, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("cannot get pod info without the pod name")
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
		Pod:        pod,
	}, nil
}

func (c *client) GetLogs(ctx context.Context, req GetPodLogsRequest) (*GetPodLogsResponse, error) {
	if strings.TrimSpace(req.Pod) == "" {
		return nil, errors.New("cannot get pod logs without providing a pod name")
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

// ListPods queries Kubernetes using search criteria for the given ListPodsRequest and returns a
// list of pods.
func (c *client) ListPods(ctx context.Context, req *ListPodsRequest) (*ListPodsResponse, error) {
	res := &ListPodsResponse{}

	namespace := req.Namespace
	if strings.TrimSpace(namespace) == "" {
		namespace = defaultNamespace
	}

	listOpts := metav1.ListOptions{}
	if len(req.LabelSelectors) > 0 {
		listOpts.LabelSelector = strings.Join(req.LabelSelectors, ",")
	}
	if len(req.FieldSelectors) > 0 {
		listOpts.FieldSelector = strings.Join(req.FieldSelectors, ",")
	}

	req.Retrier.Func = func(queryCtx context.Context) (any, error) {
		return c.clientset.CoreV1().Pods(namespace).List(ctx, listOpts)
	}

	result, err := retry.Retry(ctx, req.Retrier)
	if err != nil {
		return res, fmt.Errorf("listing pods: %w", err)
	}

	pods, ok := result.(*v1.PodList)
	if !ok {
		return res, fmt.Errorf("listing pods: unexpected response type: %v", result)
	}

	p := Pods(*pods)
	res.Pods = &p

	return res, nil
}

// CoreV1 returns the clients CoreV1 client.
func (c *client) CoreV1() cv1.CoreV1Interface {
	return c.clientset.CoreV1()
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
// kubeconfig path in the users' home dir (~/.kube/config).
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
	if pod.Spec.Containers == nil {
		return nil
	}

	containers := make([]string, len(pod.Spec.Containers))
	for i := range pod.Spec.Containers {
		containers[i] = pod.Spec.Containers[i].Name
	}

	return containers
}

// String returns the list of pods as a string.
func (r *ListPodsResponse) String() string {
	if r == nil || r.Pods == nil || r.Pods.Items == nil || len(r.Pods.Items) < 1 {
		return ""
	}

	return r.Pods.String()
}

// String returns the pods list as a human readable string.
func (p *Pods) String() string {
	if p == nil || p.Items == nil || len(p.Items) < 1 {
		return ""
	}

	out := new(strings.Builder)

	for i := range p.Items {
		i := i
		out.WriteString(p.Items[i].ObjectMeta.Name + "\n")
		out.WriteString(fmt.Sprintf("  Name: %s\n", p.Items[i].ObjectMeta.Name))
		out.WriteString(fmt.Sprintf("  Namespace: %s\n", p.Items[i].ObjectMeta.Namespace))
		out.WriteString(fmt.Sprintf("  Node Name: %s\n", p.Items[i].Spec.NodeName))
		out.WriteString(fmt.Sprintf("  Hostname: %s\n", p.Items[i].Spec.Hostname))
		out.WriteString(fmt.Sprintf("  Subdomain: %s\n", p.Items[i].Spec.Subdomain))
		out.WriteString(fmt.Sprintf("  Resource Version: %s\n", p.Items[i].ObjectMeta.ResourceVersion))
		out.WriteString(fmt.Sprintf("  Generation: %d\n", p.Items[i].ObjectMeta.Generation))
		out.WriteString(fmt.Sprintf("  Creation Timestamp: %q\n", p.Items[i].ObjectMeta.CreationTimestamp))

		if p.Items[i].Spec.OS != nil {
			out.WriteString(fmt.Sprintf("  OS: %s\n", p.Items[i].Spec.OS.Name))
		}

		out.WriteString(fmt.Sprintf("  Phase: %s\n", p.Items[i].Status.Phase))
		out.WriteString(fmt.Sprintf("  Message: %s\n", p.Items[i].Status.Message))
		out.WriteString(fmt.Sprintf("  Reason: %s\n", p.Items[i].Status.Reason))
		out.WriteString(fmt.Sprintf("  Scheduler Name: %s\n", p.Items[i].Spec.SchedulerName))
		out.WriteString(fmt.Sprintf("  IP: %s\n", p.Items[i].Status.PodIP))

		labels := p.Items[i].ObjectMeta.GetLabels()
		if len(labels) > 0 {
			out.WriteString(fmt.Sprintln("  Labels:"))
			for k, v := range labels {
				out.WriteString(fmt.Sprintf("    %s=%s\n", k, v))
			}
		}

		for ic := range p.Items[i].Spec.InitContainers {
			ic := ic
			if ic == 0 {
				out.WriteString(fmt.Sprintln("  Init Containers:"))
			}
			out.WriteString(istrings.Indent("    ", containerToString(p.Items[i].Spec.InitContainers[ic])))
		}

		for ic := range p.Items[i].Status.InitContainerStatuses {
			ic := ic
			if ic == 0 {
				out.WriteString(fmt.Sprintln("  Init Container Status:"))
			}
			out.WriteString(istrings.Indent("    ", contianerStatusToString(p.Items[i].Status.InitContainerStatuses[ic])))
		}

		for ic := range p.Items[i].Spec.Containers {
			ic := ic
			if ic == 0 {
				out.WriteString(fmt.Sprintln("  Containers:"))
			}
			out.WriteString(istrings.Indent("    ", containerToString(p.Items[i].Spec.Containers[ic])))
		}

		for ic := range p.Items[i].Status.ContainerStatuses {
			ic := ic
			if ic == 0 {
				out.WriteString(fmt.Sprintln("  Container Status:"))
			}
			out.WriteString(istrings.Indent("    ", contianerStatusToString(p.Items[i].Status.ContainerStatuses[ic])))
		}

		for ic := range p.Items[i].Spec.EphemeralContainers {
			ic := ic
			if ic == 0 {
				out.WriteString(fmt.Sprintln("  Ephemeral Containers:"))
			}

			// EphemeralContainerCommon has all the same fields as Container. We'll convert it
			// to use our stringer.
			bytes, err := p.Items[i].Spec.EphemeralContainers[ic].EphemeralContainerCommon.Marshal()
			if err != nil {
				continue
			}
			container := &v1.Container{}
			err = container.Unmarshal(bytes)
			if err != nil {
				continue
			}

			out.WriteString(istrings.Indent("    ", containerToString(*container)))
		}

		for ic := range p.Items[i].Status.EphemeralContainerStatuses {
			ic := ic
			if ic == 0 {
				out.WriteString(fmt.Sprintln("  Ephemeral Container Status:"))
			}
			out.WriteString(istrings.Indent("    ", contianerStatusToString(p.Items[i].Status.EphemeralContainerStatuses[ic])))
		}
	}

	return out.String()
}

func containerToString(container v1.Container) string {
	out := new(strings.Builder)
	out.WriteString(container.Name + "\n")
	out.WriteString(fmt.Sprintf("  Name: %s\n", container.Name))
	out.WriteString(fmt.Sprintf("  Image: %s\n", container.Image))
	out.WriteString(fmt.Sprintf("  Command: %s\n", strings.Join(container.Command, " ")))
	out.WriteString(fmt.Sprintln("  Args:"))
	for c := range container.Args {
		c := c
		out.WriteString(istrings.Indent("    ", container.Args[c]))
	}
	out.WriteString(fmt.Sprintln(""))
	out.WriteString(fmt.Sprintln("  Ports:"))
	for c := range container.Ports {
		c := c
		out.WriteString(fmt.Sprintf("    %s:\n", container.Ports[c].Name))
		out.WriteString(fmt.Sprintf("      Host Port: %d\n", container.Ports[c].HostPort))
		out.WriteString(fmt.Sprintf("      Container Port: %d\n", container.Ports[c].ContainerPort))
		out.WriteString(fmt.Sprintf("      Protocol: %s\n", container.Ports[c].Protocol))
		out.WriteString(fmt.Sprintf("      Host IP: %s\n", container.Ports[c].HostIP))
	}
	out.WriteString(fmt.Sprintln("  Environment:"))
	for c := range container.Env {
		c := c
		name := container.Env[c].Name
		value := container.Env[c].Value
		if source := container.Env[c].ValueFrom; source != nil {
			sourceStr := ""
			if source.FieldRef != nil {
				sourceStr = fmt.Sprintf("%s:%s", source.FieldRef.APIVersion, source.FieldRef.FieldPath)
			}
			if source.ResourceFieldRef != nil {
				sourceStr = fmt.Sprintf("%s:%s", source.ResourceFieldRef.ContainerName, source.ResourceFieldRef.Resource)
			}
			if source.ConfigMapKeyRef != nil {
				sourceStr = fmt.Sprintf("%s:%s", source.ConfigMapKeyRef.LocalObjectReference.Name, source.ConfigMapKeyRef.Key)
			}
			if source.SecretKeyRef != nil {
				sourceStr = fmt.Sprintf("%s:%s", source.SecretKeyRef.LocalObjectReference.Name, source.SecretKeyRef.Key)
			}
			if value == "" {
				value = fmt.Sprintf("(%s)", sourceStr)
			} else {
				value = fmt.Sprintf("%s (%s)", value, sourceStr)
			}
		}
		if name == "VAULT_LICENSE" {
			value = "[redacted]"
		}
		out.WriteString(fmt.Sprintf("    %s=%s\n", name, value))
	}

	return out.String()
}

func contianerStatusToString(status v1.ContainerStatus) string {
	out := new(strings.Builder)
	out.WriteString(status.Name + "\n")
	out.WriteString(fmt.Sprintf("  Name: %s\n", status.Name))
	out.WriteString(fmt.Sprintf("  Image: %s\n", status.Image))
	out.WriteString(fmt.Sprintf("  Image ID: %s\n", status.ImageID))
	out.WriteString(fmt.Sprintf("  Container ID: %s\n", status.ContainerID))
	out.WriteString(fmt.Sprintf("  Ready: %t\n", status.Ready))
	out.WriteString(fmt.Sprintf("  Restart Count: %d\n", status.RestartCount))

	return out.String()
}
