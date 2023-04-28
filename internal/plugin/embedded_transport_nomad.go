package plugin

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	nomadapi "github.com/hashicorp/enos-provider/internal/nomad"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/nomad"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type nomadClientFactory func(cfg nomadapi.ClientCfg) (nomadapi.Client, error)

type nomadTransportBuilder func(state *embeddedTransportNomadv1, ctx context.Context) (it.Transport, error)

var defaultNomadTransportBuilder = func(state *embeddedTransportNomadv1, ctx context.Context) (it.Transport, error) {
	opts := nomad.TransportOpts{}

	if host, ok := state.Host.Get(); ok {
		opts.Host = host
	}
	if allocationID, ok := state.AllocationID.Get(); ok {
		opts.AllocationID = allocationID
	}
	if secretID, ok := state.SecretID.Get(); ok {
		opts.SecretID = secretID
	}
	if taskName, ok := state.TaskName.Get(); ok {
		opts.TaskName = taskName
	}

	return nomad.NewTransport(opts)
}

var nomadAttributes = []string{"host", "secret_id", "allocation_id", "task_name"}

var nomadTransportTmpl = template.Must(template.New("nomad_transport").Parse(`
	nomad = {
      {{range $key, $val := .}}
      {{if $val.Value}}
      {{$key}} = "{{$val.Value}}"
      {{end}}
      {{end}}
	}`))

type embeddedTransportNomadv1 struct {
	nomadTransportBuilder nomadTransportBuilder
	nomadClientFactory    nomadClientFactory

	Host         *tfString
	SecretID     *tfString
	AllocationID *tfString
	TaskName     *tfString

	// Values required for the same reason as stated in the embeddedTransportSSHv1.Values field
	Values map[string]tftypes.Value
}

func newEmbeddedTransportNomadv1() *embeddedTransportNomadv1 {
	return &embeddedTransportNomadv1{
		nomadTransportBuilder: defaultNomadTransportBuilder,
		nomadClientFactory:    nomadapi.NewClient,
		Host:                  newTfString(),
		SecretID:              newTfString(),
		AllocationID:          newTfString(),
		TaskName:              newTfString(),
		Values:                map[string]tftypes.Value{},
	}
}

var _ transportState = (*embeddedTransportNomadv1)(nil)

func (em *embeddedTransportNomadv1) Terraform5Type() tftypes.Type {
	return terraform5Type(em.Values)
}

func (em *embeddedTransportNomadv1) Terraform5Value() tftypes.Value {
	// If the values are empty it means that the transport configuration is unknown
	if len(em.Values) == 0 {
		return tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"host":          tftypes.String,
			"secret_id":     tftypes.String,
			"allocation_id": tftypes.String,
			"task_name":     tftypes.String,
		}}, tftypes.UnknownValue)
	}

	return terraform5Value(em.Values)
}

func (em *embeddedTransportNomadv1) ApplyDefaults(defaults map[string]TFType) error {
	return applyDefaults(defaults, em.Attributes())
}

func (em *embeddedTransportNomadv1) CopyValues() map[string]tftypes.Value {
	return copyValues(em.Values)
}

func (em *embeddedTransportNomadv1) IsConfigured() bool {
	return isTransportConfigured(em)
}

func (em *embeddedTransportNomadv1) FromTerraform5Value(val tftypes.Value) (err error) {
	em.Values, err = mapAttributesTo(val, map[string]interface{}{
		"host":          em.Host,
		"secret_id":     em.SecretID,
		"allocation_id": em.AllocationID,
		"task_name":     em.TaskName,
	})
	if err != nil {
		return AttributePathError(
			fmt.Errorf("failed to convert terraform value to 'Nomad' transport config, due to: %w", err),
			"transport", "nomad",
		)
	}

	return verifyConfiguration(nomadAttributes, em.Values, "nomad")
}

func (em *embeddedTransportNomadv1) Validate(ctx context.Context) error {
	for name, prop := range map[string]*tfString{
		"host":          em.Host,
		"allocation_id": em.AllocationID,
		"task_name":     em.TaskName,
	} {
		if _, ok := prop.Get(); !ok {
			return ValidationError(
				fmt.Sprintf("missing value for required attribute: %s", name),
				"transport", "nomad", name,
			)
		}
	}

	return nil
}

func (em *embeddedTransportNomadv1) Client(ctx context.Context) (it.Transport, error) {
	return em.nomadTransportBuilder(em, ctx)
}

func (em *embeddedTransportNomadv1) Attributes() map[string]TFType {
	return map[string]TFType{
		"host":          em.Host,
		"secret_id":     em.SecretID,
		"allocation_id": em.AllocationID,
		"task_name":     em.TaskName,
	}
}

func (em *embeddedTransportNomadv1) GetAttributesForReplace() []string {
	var attribsForReplace []string
	if _, ok := em.Values["host"]; ok {
		attribsForReplace = append(attribsForReplace, "host")
	}

	if _, ok := em.Values["secret_id"]; ok {
		attribsForReplace = append(attribsForReplace, "secret_id")
	}

	return attribsForReplace
}

func (em *embeddedTransportNomadv1) Type() TransportType {
	return NOMAD
}

func (em *embeddedTransportNomadv1) render() (string, error) {
	buf := bytes.Buffer{}
	if err := nomadTransportTmpl.Execute(&buf, em.Attributes()); err != nil {
		return "", fmt.Errorf("failed to render nomad transport config, due to: %w", err)
	}

	return buf.String(), nil
}

func (em *embeddedTransportNomadv1) debug() string {
	maxWidth := 0
	attributes := em.Attributes()
	for name := range attributes {
		if len(name) > maxWidth {
			maxWidth = len(name)
		}
	}

	vals := make([]string, len(nomadAttributes))
	for i, name := range nomadAttributes {
		val := "null"
		if value, ok := attributes[name]; ok && !value.TFValue().IsNull() {
			if name == "secret_id" {
				val = "[redacted]"
			} else {
				val = value.String()
			}
		}
		vals[i] = fmt.Sprintf("%*s : %s", maxWidth, name, val)
	}

	return fmt.Sprintf("Nomad Transport Config:\n%s", strings.Join(vals, "\n"))
}

// nomadClient creates a nomad client for this transport.
func (em *embeddedTransportNomadv1) nomadClient() (nomadapi.Client, error) {
	client, err := em.nomadClientFactory(nomadapi.ClientCfg{
		Host:     em.Host.Val,
		SecretID: em.SecretID.Val,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create nomad client due to: %w", err)
	}

	return client, nil
}
