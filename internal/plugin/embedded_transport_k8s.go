package plugin

import (
	"context"
	"fmt"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/k8s"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type k8sTransportBuilder func(state *embeddedTransportK8Sv1, ctx context.Context) (it.Transport, error)

var defaultK8STransportBuilder = func(state *embeddedTransportK8Sv1, ctx context.Context) (it.Transport, error) {
	opts := k8s.TransportOpts{}

	if err := state.Validate(ctx); err != nil {
		return nil, err
	}

	if kubeConfig, ok := state.KubeConfigBase64.Get(); ok {
		opts.KubeConfigBase64 = kubeConfig
	}
	if contextName, ok := state.ContextName.Get(); ok {
		opts.ContextName = contextName
	}
	if namespace, ok := state.Namespace.Get(); ok {
		opts.Namespace = namespace
	}
	if pod, ok := state.Pod.Get(); ok {
		opts.Pod = pod
	}
	if container, ok := state.Container.Get(); ok {
		opts.Container = container
	}

	return k8s.NewTransport(opts)
}

var k8sAttributes = []string{"kubeconfig_base64", "context_name", "namespace", "pod", "container"}

type embeddedTransportK8Sv1 struct {
	k8sTransportBuilder k8sTransportBuilder // added in order to support testing

	KubeConfigBase64 *tfString
	ContextName      *tfString
	Namespace        *tfString
	Pod              *tfString
	Container        *tfString

	// Values required for the same reason as stated in the embeddedTransportSSHv1.Values field
	Values map[string]tftypes.Value
}

var _ transportState = (*embeddedTransportK8Sv1)(nil)

func newEmbeddedTransportK8Sv1() *embeddedTransportK8Sv1 {
	return &embeddedTransportK8Sv1{
		k8sTransportBuilder: defaultK8STransportBuilder,
		KubeConfigBase64:    newTfString(),
		ContextName:         newTfString(),
		Namespace:           newTfString(),
		Pod:                 newTfString(),
		Container:           newTfString(),
		Values:              map[string]tftypes.Value{},
	}
}

// Terraform5Type is the dynamically generated K8S tftypes.Type. It must
// always match the schema that is passed in as user configuration.
func (em *embeddedTransportK8Sv1) Terraform5Type() tftypes.Type {
	return terraform5Type(em.Values)
}

func (em *embeddedTransportK8Sv1) Terraform5Value() tftypes.Value {
	return terraform5Value(em.Values)
}

func (em *embeddedTransportK8Sv1) ApplyDefaults(defaults map[string]TFType) error {
	return applyDefaults(defaults, em.Attributes())
}

func (em *embeddedTransportK8Sv1) CopyValues() map[string]tftypes.Value {
	return copyValues(em.Values)
}

func (em *embeddedTransportK8Sv1) IsConfigured() bool {
	return isTransportConfigured(em)
}

func (em *embeddedTransportK8Sv1) FromTerraform5Value(val tftypes.Value) (err error) {
	em.Values, err = mapAttributesTo(val, map[string]interface{}{
		"kubeconfig_base64": em.KubeConfigBase64,
		"context_name":      em.ContextName,
		"namespace":         em.Namespace,
		"pod":               em.Pod,
		"container":         em.Container,
	})
	if err != nil {
		return wrapErrWithDiagnostics(err, "invalid configuration syntax",
			"unable to marshal transport Kubernetes values", "transport", "kubernetes",
		)
	}
	return verifyConfiguration(k8sAttributes, em.Values, "kubernetes")
}

func (em *embeddedTransportK8Sv1) Validate(ctx context.Context) error {
	for name, prop := range map[string]*tfString{
		"kubeconfig_base64": em.KubeConfigBase64,
		"context_name":      em.ContextName,
		"pod":               em.Pod,
	} {
		if _, ok := prop.Get(); !ok {
			return newErrWithDiagnostics("Invalid Transport Configuration", fmt.Sprintf("missing value for required attribute: %s", name), "transport", "kubernetes", name)
		}
	}
	return nil
}

func (em *embeddedTransportK8Sv1) Client(ctx context.Context) (it.Transport, error) {
	return em.k8sTransportBuilder(em, ctx)
}

func (em *embeddedTransportK8Sv1) Attributes() map[string]TFType {
	return map[string]TFType{
		"kubeconfig_base64": em.KubeConfigBase64,
		"context_name":      em.ContextName,
		"namespace":         em.Namespace,
		"pod":               em.Pod,
		"container":         em.Container,
	}
}

func (em *embeddedTransportK8Sv1) GetAttributesForReplace() []string {
	var attribsForReplace []string
	if _, ok := em.Values["kubeconfig_base64"]; ok {
		attribsForReplace = append(attribsForReplace, "kubeconfig_base64")
	}

	if _, ok := em.Values["context_name"]; ok {
		attribsForReplace = append(attribsForReplace, "context_name")
	}

	return attribsForReplace
}
