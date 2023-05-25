package plugin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/enos-provider/internal/diags"
	"github.com/hashicorp/enos-provider/internal/kubernetes"
	"github.com/hashicorp/enos-provider/internal/server/datarouter"
	"github.com/hashicorp/enos-provider/internal/server/state"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const defaultWaitTimeout = time.Minute

type podInfoGetter func(ctx context.Context, state kubernetesPodsStateV1) ([]kubernetes.PodInfo, error)

var defaultPodInfoGetter podInfoGetter = func(ctx context.Context, state kubernetesPodsStateV1) ([]kubernetes.PodInfo, error) {
	client, err := kubernetes.NewClient(kubernetes.ClientCfg{
		KubeConfigBase64: state.KubeConfigBase64.Value(),
		ContextName:      state.ContextName.Value(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create a Kubernetes Client, due to: %w", err)
	}

	request := kubernetes.QueryPodInfosRequest{
		Namespace:     state.Namespace.Value(),
		LabelSelector: strings.Join(state.LabelSelectors.StringValue(), ","),
		FieldSelector: strings.Join(state.FieldSelectors.StringValue(), ","),
	}

	if count, ok := state.ExpectedPodCount.Get(); ok {
		request.ExpectedPodCount = count
	}

	if timeout, ok := state.WaitTimeout.Get(); ok {
		timeout, err := time.ParseDuration(timeout)
		if err != nil {
			return nil, fmt.Errorf("failed to parse 'wait_timeout': %s", timeout)
		}
		request.WaitTimeout = timeout
	} else {
		request.WaitTimeout = defaultWaitTimeout
	}

	pods, err := client.QueryPodInfos(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to query pods, due to: %w", err)
	}

	return pods, nil
}

type kubernetesPods struct {
	providerConfig *config
	podInfoGetter  podInfoGetter
}

var _ datarouter.DataSource = (*kubernetesPods)(nil)

type kubernetesPodsStateV1 struct {
	ID               *tfString
	KubeConfigBase64 *tfString
	ContextName      *tfString
	Namespace        *tfString
	LabelSelectors   *tfStringSlice
	FieldSelectors   *tfStringSlice
	Pods             *tfObjectSlice
	Transports       *tfObjectSlice
	ExpectedPodCount *tfNum
	WaitTimeout      *tfString

	failureHandlers
}

var _ state.State = (*kubernetesPodsStateV1)(nil)

func newKubernetesPods() *kubernetesPods {
	return &kubernetesPods{
		providerConfig: newProviderConfig(),
		podInfoGetter:  defaultPodInfoGetter,
	}
}

func newKubernetesPodStateV1() *kubernetesPodsStateV1 {
	pods := newTfObjectSlice()
	pods.AttrTypes = map[string]tftypes.Type{
		"name":      tftypes.String,
		"namespace": tftypes.String,
		"containers": tftypes.List{
			ElementType: tftypes.String,
		},
	}

	transports := newTfObjectSlice()
	transports.AttrTypes = map[string]tftypes.Type{
		"kubeconfig_base64": tftypes.String,
		"context_name":      tftypes.String,
		"namespace":         tftypes.String,
		"pod":               tftypes.String,
		"container":         tftypes.String,
	}

	return &kubernetesPodsStateV1{
		ID:               newTfString(),
		KubeConfigBase64: newTfString(),
		ContextName:      newTfString(),
		Namespace:        newTfString(),
		LabelSelectors:   newTfStringSlice(),
		FieldSelectors:   newTfStringSlice(),
		ExpectedPodCount: newTfNum(),
		WaitTimeout:      newTfString(),
		Pods:             pods,
		Transports:       transports,
		failureHandlers:  failureHandlers{},
	}
}

func (d *kubernetesPods) Name() string {
	return "enos_kubernetes_pods"
}

func (d *kubernetesPods) Schema() *tfprotov6.Schema {
	return newKubernetesPodStateV1().Schema()
}

func (d *kubernetesPods) SetProviderConfig(meta tftypes.Value) error {
	return d.providerConfig.FromTerraform5Value(meta)
}

// ValidateDataResourceConfig is the request Terraform sends when it wants to
// validate the data source's configuration.
func (d *kubernetesPods) ValidateDataResourceConfig(ctx context.Context, req tfprotov6.ValidateDataResourceConfigRequest, res *tfprotov6.ValidateDataResourceConfigResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	// unmarshal it to our known type to ensure whatever was passed in matches
	// the correct schema.
	state := newKubernetesPodStateV1()
	err := unmarshal(state, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
	}
}

// ReadDataSource is the request Terraform sends when it wants to get the latest
// state for the data source.
func (d *kubernetesPods) ReadDataSource(ctx context.Context, req tfprotov6.ReadDataSourceRequest, res *tfprotov6.ReadDataSourceResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	podState := newKubernetesPodStateV1()

	err := unmarshal(podState, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	if err = podState.Validate(ctx); err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Validation Error", err))
		return
	}

	podState.ID.Set("static")

	pods, err := d.podInfoGetter(ctx, *podState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Pod Query Error", err))
		return
	}

	podResults := make([]*tfObject, len(pods))
	var transportResults []*tfObject
	for i := range pods {
		pod := newTfObject()
		podName := newTfString()
		podName.Set(pods[i].Name)
		podNamespace := newTfString()
		podNamespace.Set(pods[i].Namespace)

		containers := newTfStringSlice()
		containers.SetStrings(pods[i].Containers)

		pod.Set(map[string]interface{}{
			"name":       podName,
			"namespace":  podNamespace,
			"containers": containers,
		})
		podResults[i] = pod

		for _, c := range pods[i].Containers {
			transport := newTfObject()
			kubeConfigBase64 := newTfString()
			kubeConfigBase64.Set(podState.KubeConfigBase64.Val)
			contextName := newTfString()
			contextName.Set(podState.ContextName.Val)
			container := newTfString()
			container.Set(c)

			transport.Set(map[string]interface{}{
				"kubeconfig_base64": kubeConfigBase64,
				"context_name":      contextName,
				"namespace":         podNamespace,
				"pod":               podName,
				"container":         container,
			})

			transportResults = append(transportResults, transport)
		}
	}
	podState.Pods.Set(podResults)
	podState.Transports.Set(transportResults)

	res.State, err = state.Marshal(podState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
	}
}

// Schema the kubernetesPodsStateV1 Schema.
func (s *kubernetesPodsStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:     "id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:        "kubeconfig_base64",
					Description: "The base64 encoded kubeconfig for the cluster to connect to as a string",
					Type:        tftypes.String,
					Required:    true,
					Sensitive:   true,
				},
				{
					Name:        "context_name",
					Description: "The name of the cluster context to connect to",
					Type:        tftypes.String,
					Required:    true,
				},
				{
					Name:        "namespace",
					Description: "The namespace to query the pods in.",
					Type:        tftypes.String,
					Optional:    true,
				},
				{
					Name:        "label_selectors",
					Description: "Label selectors to use when querying pods, see https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/",
					Type:        tftypes.List{ElementType: tftypes.String},
					Optional:    true,
				},
				{
					Name:        "field_selectors",
					Description: "Field selectors to use when querying pods, see https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/",
					Type:        tftypes.List{ElementType: tftypes.String},
					Optional:    true,
				},
				{
					Name:        "expected_pod_count",
					Description: "The number of pods that are expected to be found matching the query.",
					Type:        tftypes.Number,
					Optional:    true,
				},
				{
					Name:        "wait_timeout",
					Description: "The amount of time to wait for the pods found in the query to be in the 'Running' state. If not provided a default of 1m will be used.",
					Type:        tftypes.String,
					Optional:    true,
				},
				{
					Name:        "pods",
					Description: "A list of PodInfo objects for all the pods that match the search.",
					Type:        s.Pods.TFType(),
					Computed:    true,
				},
				{
					Name: "transports",
					Description: "A list of transport blocks for all the pods (and their containers) that match the search. " +
						"The values can be used for the transport argument of any resource that requires Kubernetes transport.",
					Type:     s.Transports.TFType(),
					Computed: true,
				},
			},
		},
	}
}

// Validate validates the configuration.
func (s *kubernetesPodsStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	kubeconfig, ok := s.KubeConfigBase64.Get()
	if !ok {
		// this should never happen, since 'kubeconfig' is a required field and Terraform would barf
		// before passing the request to the provider
		return ValidationError("cannot query pods without a 'kubeconfig_base64'", "kubeconfig_base64")
	}

	contextName, ok := s.ContextName.Get()
	if !ok {
		// this should never happen, since 'context_name' is a required field and Terraform would barf
		// before passing the request to the provider
		return ValidationError("cannot query pods without a 'context_name'", "context_name")
	}

	kubeConfig, err := kubernetes.DecodeAndLoadKubeConfig(kubeconfig)
	if err != nil {
		return ValidationError("invalid kubeconfig, kubeconfig should be a valid base64 encoded kubeconfig string", "kubeconfig_base64")
	}

	if timeout, ok := s.WaitTimeout.Get(); ok {
		_, err := time.ParseDuration(timeout)
		if err != nil {
			return ValidationError(fmt.Sprintf("failed to parse duration [%s]", timeout), "wait_timeout")
		}
	}

	if count, ok := s.ExpectedPodCount.Get(); ok {
		if count <= 0 {
			return ValidationError("expected pod count must be greater than 0", "expected_pod_count")
		}
	}

	// check if the context is exists in the provided kubeconfig
	if _, ok := kubeConfig.Contexts[contextName]; !ok {
		return ValidationError(fmt.Sprintf("context: [%s] not present in the provided kubeconfig", contextName), "context_name")
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *kubernetesPodsStateV1) FromTerraform5Value(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]interface{}{
		"id":                 s.ID,
		"kubeconfig_base64":  s.KubeConfigBase64,
		"context_name":       s.ContextName,
		"namespace":          s.Namespace,
		"label_selectors":    s.LabelSelectors,
		"field_selectors":    s.FieldSelectors,
		"expected_pod_count": s.ExpectedPodCount,
		"wait_timeout":       s.WaitTimeout,
		"pods":               s.Pods,
		"transports":         s.Transports,
	})

	return err
}

// Terraform5Type is the file state tftypes.Type.
func (s *kubernetesPodsStateV1) Terraform5Type() tftypes.Type {
	// TODO: Add each state attribute
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":                 s.ID.TFType(),
		"kubeconfig_base64":  s.KubeConfigBase64.TFType(),
		"context_name":       s.ContextName.TFType(),
		"namespace":          s.Namespace.TFType(),
		"label_selectors":    s.LabelSelectors.TFType(),
		"field_selectors":    s.FieldSelectors.TFType(),
		"expected_pod_count": s.ExpectedPodCount.TFType(),
		"wait_timeout":       s.WaitTimeout.TFType(),
		"pods":               s.Pods.TFType(),
		"transports":         s.Transports.TFType(),
	}}
}

// Terraform5Value is the file state tftypes.Value.
func (s *kubernetesPodsStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":                 s.ID.TFValue(),
		"kubeconfig_base64":  s.KubeConfigBase64.TFValue(),
		"context_name":       s.ContextName.TFValue(),
		"namespace":          s.Namespace.TFValue(),
		"label_selectors":    s.LabelSelectors.TFValue(),
		"field_selectors":    s.FieldSelectors.TFValue(),
		"expected_pod_count": s.ExpectedPodCount.TFValue(),
		"wait_timeout":       s.WaitTimeout.TFValue(),
		"pods":               s.Pods.TFValue(),
		"transports":         s.Transports.TFValue(),
	})
}
