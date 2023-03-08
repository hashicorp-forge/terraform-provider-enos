package plugin

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/hashicorp/enos-provider/internal/kubernetes"
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

var k8sTransportTmpl = template.Must(template.New("k8s_transport").Parse(`
    kubernetes = {
      {{range $key, $val := .}}
      {{if $val.Value}}
      {{$key}} = "{{$val.Value}}"
      {{end}}
      {{end}}
    }`))

type embeddedTransportK8Sv1 struct {
	k8sTransportBuilder k8sTransportBuilder // added in order to support testing
	k8sClientFactory    func(cfg kubernetes.ClientCfg) (kubernetes.Client, error)

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
		k8sClientFactory:    kubernetes.NewClient,
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
		return AttributePathError(
			fmt.Errorf("failed to convert terraform value to 'Kubernetes' transport config, due to: %w", err),
			"transport", "kubernetes",
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
			return ValidationError(
				fmt.Sprintf("missing value for required attribute: %s", name),
				"transport", "kubernetes", name,
			)
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

func (em *embeddedTransportK8Sv1) Type() TransportType {
	return K8S
}

// render renders the transport to terraform
func (em *embeddedTransportK8Sv1) render() (string, error) {
	buf := bytes.Buffer{}
	if err := k8sTransportTmpl.Execute(&buf, em.Attributes()); err != nil {
		return "", fmt.Errorf("failed to render k8s transport config, due to: %w", err)
	}

	return buf.String(), nil
}

func (em *embeddedTransportK8Sv1) debug() string {
	maxWidth := 0
	attributes := em.Attributes()
	for name := range attributes {
		if len(name) > maxWidth {
			maxWidth = len(name)
		}
	}

	var vals []string
	for _, name := range k8sAttributes {
		val := "null"
		if value, ok := attributes[name]; ok && !value.TFValue().IsNull() {
			if name == "kubeconfig_base64" {
				val = "[redacted]"
			} else {
				val = value.String()
			}
		}
		vals = append(vals, fmt.Sprintf("%*s : %s", maxWidth, name, val))
	}

	return fmt.Sprintf("Kubernetes Transport Config:\n%s", strings.Join(vals, "\n"))
}

func (em *embeddedTransportK8Sv1) k8sClient() (kubernetes.Client, error) {
	cfg := kubernetes.ClientCfg{}

	kubeconfig, ok := em.KubeConfigBase64.Get()
	if !ok {
		return nil, fmt.Errorf("failed to create kubernetes client, 'kubeconfig_base64' was not configured")
	}
	cfg.KubeConfigBase64 = kubeconfig

	contextName, ok := em.ContextName.Get()
	if !ok {
		return nil, fmt.Errorf("failed to create kubernetes client, 'context_name' was not configured")
	}
	cfg.ContextName = contextName

	return em.k8sClientFactory(cfg)
}
