package plugin

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/enos-provider/internal/diags"
	"github.com/hashicorp/enos-provider/internal/kubernetes"
	"github.com/hashicorp/enos-provider/internal/server/datarouter"
	"github.com/hashicorp/enos-provider/internal/server/state"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type podInfoGetter func(ctx context.Context, state kubernetesPodsStateV1) ([]kubernetes.PodInfo, error)

var defaultPodInfoGetter podInfoGetter = func(ctx context.Context, state kubernetesPodsStateV1) ([]kubernetes.PodInfo, error) {
	client, err := kubernetes.NewClient(kubernetes.ClientCfg{
		KubeConfigBase64: state.KubeConfigBase64.Value(),
		ContextName:      state.ContextName.Value(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create a Kubernetes Client, due to: %w", err)
	}

	pods, err := client.GetPodInfos(ctx, kubernetes.GetPodInfoRequest{
		Namespace:     state.Namespace.Value(),
		LabelSelector: strings.Join(state.LabelSelectors.StringValue(), ","),
		FieldSelector: strings.Join(state.FieldSelectors.StringValue(), ","),
	})
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
	}

	return &kubernetesPodsStateV1{
		ID:               newTfString(),
		KubeConfigBase64: newTfString(),
		ContextName:      newTfString(),
		Namespace:        newTfString(),
		LabelSelectors:   newTfStringSlice(),
		FieldSelectors:   newTfStringSlice(),
		Pods:             pods,
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

	var podResults []*tfObject
	for _, result := range pods {
		r := newTfObject()
		podName := newTfString()
		podName.Set(result.Name)
		podNamespace := newTfString()
		podNamespace.Set(result.Namespace)

		r.Set(map[string]interface{}{
			"name":      podName,
			"namespace": podNamespace,
		})
		podResults = append(podResults, r)
	}

	podState.Pods.Set(podResults)

	res.State, err = state.Marshal(podState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
	}
}

// Schema the kubernetesPodsStateV1 Schema
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
					Description: "The name of the cluster context top connect to",
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
					Name:        "pods",
					Description: "A map of namespaces to pod names for the pods that match the search criteria.",
					Type:        s.Pods.TFType(),
					Computed:    true,
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

	// check if the context is exists in the provided kubeconfig
	if _, ok := kubeConfig.Contexts[contextName]; !ok {
		return ValidationError(fmt.Sprintf("context: [%s] not present in the provided kubeconfig", contextName), "context_name")
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *kubernetesPodsStateV1) FromTerraform5Value(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]interface{}{
		"id":                s.ID,
		"kubeconfig_base64": s.KubeConfigBase64,
		"context_name":      s.ContextName,
		"namespace":         s.Namespace,
		"label_selectors":   s.LabelSelectors,
		"field_selectors":   s.FieldSelectors,
		"pods":              s.Pods,
	})

	return err
}

// Terraform5Type is the file state tftypes.Type.
func (s *kubernetesPodsStateV1) Terraform5Type() tftypes.Type {
	// TODO: Add each state attribute
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":                s.ID.TFType(),
		"kubeconfig_base64": s.KubeConfigBase64.TFType(),
		"context_name":      s.ContextName.TFType(),
		"namespace":         s.Namespace.TFType(),
		"label_selectors":   s.LabelSelectors.TFType(),
		"field_selectors":   s.FieldSelectors.TFType(),
		"pods":              s.Pods.TFType(),
	}}
}

// Terraform5Value is the file state tftypes.Value.
func (s *kubernetesPodsStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":                s.ID.TFValue(),
		"kubeconfig_base64": s.KubeConfigBase64.TFValue(),
		"context_name":      s.ContextName.TFValue(),
		"namespace":         s.Namespace.TFValue(),
		"label_selectors":   s.LabelSelectors.TFValue(),
		"field_selectors":   s.FieldSelectors.TFValue(),
		"pods":              s.Pods.TFValue(),
	})
}
