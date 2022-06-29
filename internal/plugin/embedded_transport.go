package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// embeddedTransportV1 represents the embedded transport state for all
// resources and data source. It is intended to be used as ouput from the
// transport data source and used as the transport input for all resources.
type embeddedTransportV1 struct {
	mu  sync.Mutex
	SSH *embeddedTransportSSHv1 `json:"ssh"`
	K8S *embeddedTransportK8Sv1 `json:"k8s"`
}

// transportState interface defining the api of a transport
type transportState interface {
	Serializable

	// ApplyDefaults sets values from the provided defaults if they have not already been configured.
	ApplyDefaults(defaults map[string]TFType) error

	// CopyValues create a copy of the Values map for this transport
	CopyValues() map[string]tftypes.Value

	// Attributes returns all the attributes of the transport state. This includes those that where
	// configured by the user (via terraform configuration) and those that received their values
	// via defaults
	Attributes() map[string]TFType

	// Validate checks that the configuration is valid for this transport
	Validate(ctx context.Context) error

	// Client builds a client for this transport
	Client(ctx context.Context) (it.Transport, error)

	// GetAttributesForReplace gets the attributes that if changed should trigger a replace of the resource
	GetAttributesForReplace() []string

	// IsConfigured returns true if the transport has been configured, false otherwise. A configured
	// transport is one that has a value for any of its attributes. The attribute value can come either
	// from the resources terraform configuration or from provider defaults.
	IsConfigured() bool
}

func newEmbeddedTransport() *embeddedTransportV1 {
	return &embeddedTransportV1{
		mu:  sync.Mutex{},
		SSH: newEmbeddedTransportSSH(),
		K8S: newEmbeddedTransportK8Sv1(),
	}
}

// SchemaAttributeTransport is our transport schema configuration attribute.
// Resources that embed a transport should use this as transport schema.
func (em *embeddedTransportV1) SchemaAttributeTransport() *tfprotov6.SchemaAttribute {
	return &tfprotov6.SchemaAttribute{
		Name:     "transport",
		Type:     em.Terraform5Type(),
		Optional: true, // We'll handle our own schema validation
	}
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Value with As().
func (em *embeddedTransportV1) FromTerraform5Value(val tftypes.Value) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	vals := map[string]tftypes.Value{}
	err := val.As(&vals)
	if err != nil {
		return err
	}

	if ssh, ok := vals["ssh"]; ok && ssh.IsKnown() {
		if err := em.SSH.FromTerraform5Value(vals["ssh"]); err != nil {
			return err
		}
	}

	if k8s, ok := vals["kubernetes"]; ok && k8s.IsKnown() {
		if err := em.K8S.FromTerraform5Value(vals["kubernetes"]); err != nil {
			return err
		}
	}

	return nil
}

// Terraform5Type is the tftypes.Type
func (em *embeddedTransportV1) Terraform5Type() tftypes.Type {
	return tftypes.DynamicPseudoType
}

// Terraform5Value generates a tftypes.Value. This must return a value that is structurally the same as
// the value unmarshalled via FromTerraform5Value. Do not try to add or remove attributes, Terraform
// does not like that.
func (em *embeddedTransportV1) Terraform5Value() tftypes.Value {
	values := map[string]tftypes.Value{}
	attributeTypes := map[string]tftypes.Type{}

	if em.SSH.IsConfigured() {
		value := em.SSH.Terraform5Value()
		values["ssh"] = value
		attributeTypes["ssh"] = value.Type()
	}

	if em.K8S.IsConfigured() {
		value := em.K8S.Terraform5Value()
		values["kubernetes"] = value
		attributeTypes["kubernetes"] = value.Type()
	}

	if len(values) == 0 {
		return tftypes.NewValue(em.Terraform5Type(), nil)
	}

	return tftypes.NewValue(tftypes.Object{AttributeTypes: attributeTypes}, values)
}

// Copy makes an identical copy of the transport. The copy will not share any of the data structures
// of this transport and is safe to modify.
func (em *embeddedTransportV1) Copy() (*embeddedTransportV1, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	newCopy := newEmbeddedTransport()

	self, err := json.Marshal(em)
	if err != nil {
		return newCopy, err
	}

	err = json.Unmarshal(self, newCopy)
	if err != nil {
		return newCopy, err
	}

	if em.SSH.IsConfigured() {
		newCopy.SSH.Values = em.SSH.CopyValues()
	}

	if em.K8S.IsConfigured() {
		newCopy.K8S.Values = em.K8S.CopyValues()
	}

	return newCopy, nil
}

// CopyValues returns only the values that have received their configuration from the
// terraform configuration for that resoruce. The configured attributes do not include the attributes
// that may have been received as default values via ApplyDefaults. To get all the attribute values
// including those recieved via defaults use the Attributes method instead.
func (em *embeddedTransportV1) CopyValues() map[string]map[string]tftypes.Value {
	configuredAttributes := map[string]map[string]tftypes.Value{}
	if em.SSH.IsConfigured() {
		configuredAttributes["ssh"] = em.SSH.CopyValues()
	}
	if em.K8S.IsConfigured() {
		configuredAttributes["kubernetes"] = em.K8S.CopyValues()
	}

	return configuredAttributes
}

// Attributes returns all the attributes of the transport state. This includes those that where
// configured by the user (via terraform configuration) and those that received their values
// via defaults
func (em *embeddedTransportV1) Attributes() map[string]map[string]TFType {
	attributes := map[string]map[string]TFType{}
	if em.SSH.IsConfigured() {
		attributes["ssh"] = em.SSH.Attributes()
	}
	if em.K8S.IsConfigured() {
		attributes["kubernetes"] = em.K8S.Attributes()
	}

	return attributes
}

// Validate validates that transport can use the given configuration as a
// transport. Be warned that this will read and path based configuration and
// attempt to parse any keys.
func (em *embeddedTransportV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	transport, err := em.GetConfiguredTransport()
	if err != nil {
		return err
	}

	return transport.Validate(ctx)
}

// Client returns a Transport client that be used to perform actions against
// the target that has been configured.
func (em *embeddedTransportV1) Client(ctx context.Context) (it.Transport, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	transport, err := em.GetConfiguredTransport()
	if err != nil {
		return nil, err
	}
	return transport.Client(ctx)
}

// ApplyDefaults given the provided 'defaults' transport, update this transport by setting values into
// this transport that have not already been set. For example:
//
// this config     = { ssh: { host: "10.0.4.34" }}
// defaults config = { ssh: { host: "10.0.4.10", user: "ubuntu", private_key_path: "/some/path/key.pem" }, kubernetes: { context_name: "yoyo" }}
// after apply     = { ssh: { host: "10.0.4.34", user: "ubuntu", private_key_path: "/some/path/key.pem" }}}
//
// NOTES:
// 1. Configuration is ignored for transports that are not configured. In the example you can see that
// the Kubernetes configuration was ignored since this transport did not have any Kubernetes transport
// configuration.
// 2. Configuration is ignored if this transport has a value already for that attribute. In this example
// the transport already had a value for ssh.host, therefore the provided ssh.host from the defatults
// is ignored.
func (em *embeddedTransportV1) ApplyDefaults(defaults *embeddedTransportV1) error {
	defaults.mu.Lock()
	defer defaults.mu.Unlock()
	em.mu.Lock()
	defer em.mu.Unlock()

	switch {
	case em.SSH.IsConfigured():
		if err := em.SSH.ApplyDefaults(defaults.SSH.Attributes()); err != nil {
			return err
		}
	case em.K8S.IsConfigured():
		if err := em.K8S.ApplyDefaults(defaults.K8S.Attributes()); err != nil {
			return err
		}
	// the default case here is the case where the resource does not define a transport block and
	// should therefore receive its transport configuration entirely from the enos provider configuration.
	// In this case it is not valid that the enos provider transport is configured with more than one
	// transport since it would not be possible to know which transport client to build when applying
	// the resource.
	default:
		switch {
		case defaults.SSH.IsConfigured() && !defaults.K8S.IsConfigured():
			if err := em.SSH.ApplyDefaults(defaults.SSH.Attributes()); err != nil {
				return err
			}
		case !defaults.SSH.IsConfigured() && defaults.K8S.IsConfigured():
			if err := em.K8S.ApplyDefaults(defaults.K8S.Attributes()); err != nil {
				return err
			}
		case defaults.SSH.IsConfigured() && defaults.K8S.IsConfigured():
			return newErrWithDiagnostics("Invalid Transport Configuration", "Only one transport can be configured, both 'ssh' and 'kubernetes' where configured")
		default:
			return newErrWithDiagnostics("Invalid Transport Configuration", "No transport configured, one of 'ssh' or 'kubernetes' must be configured")
		}
	}

	return nil
}

// GetConfiguredTransport Gets the configured transport for this embedded transport. There should be
// only one configured transport for a resource. If there are none configured or both configured calling
// this method will return an error.
func (em *embeddedTransportV1) GetConfiguredTransport() (transportState, error) {
	sshIsConfigured := em.SSH.IsConfigured()
	k8sIsConfigured := em.K8S.IsConfigured()

	switch {
	case sshIsConfigured && k8sIsConfigured:
		return nil, newErrWithDiagnostics("Invalid configuration", "Only one transport can be configured, both 'ssh' and 'kubernetes' where configured")
	case sshIsConfigured:
		return em.SSH, nil
	case k8sIsConfigured:
		return em.K8S, nil
	default:
		return nil, newErrWithDiagnostics(
			"Invalid Transport Configuration",
			"No transport configured, one of 'ssh' or 'kubernetes' must be configured")
	}
}

func verifyConfiguration(knownAttributes []string, values map[string]tftypes.Value, transportType string) error {
	// Because the transport type is a dynamic psuedo type we have to manually ensure
	// that the user hasn't set any unknown attributes.
	isKnownAttribute := func(attr string) error {
		for _, known := range knownAttributes {
			if attr == known {
				return nil
			}
		}

		return newErrWithDiagnostics("Unsupported argument",
			fmt.Sprintf(`An argument named "%s" is not expected here.`, attr), "transport", transportType, attr,
		)
	}

	for attribute := range values {
		err := isKnownAttribute(attribute)
		if err != nil {
			return err
		}
	}
	return nil
}

func (em *embeddedTransportV1) transportReplacedAttributePaths(proposed *embeddedTransportV1) []*tftypes.AttributePath {
	attrs := []*tftypes.AttributePath{}

	if em.SSH.IsConfigured() && proposed.SSH.IsConfigured() {
		attrs = addAttributesForReplace(em.SSH, proposed.SSH, attrs, "ssh")
	}

	if em.K8S.IsConfigured() && proposed.K8S.IsConfigured() {
		attrs = addAttributesForReplace(em.K8S, proposed.K8S, attrs, "kubernetes")
	}

	if len(attrs) > 0 {
		return attrs
	}

	return nil
}

func addAttributesForReplace(priorState transportState, proposedState transportState, attrs []*tftypes.AttributePath, transportName string) []*tftypes.AttributePath {
	for _, attribute := range priorState.GetAttributesForReplace() {
		proposedValue := proposedState.Attributes()[attribute].TFValue()
		currentValue := priorState.Attributes()[attribute].TFValue()
		if !proposedValue.Equal(currentValue) {
			attrs = append(attrs, tftypes.NewAttributePathWithSteps([]tftypes.AttributePathStep{
				tftypes.AttributeName("transport"),
				tftypes.AttributeName(transportName),
				tftypes.AttributeName(attribute),
			}))
		}
	}
	return attrs
}

func copyValues(values map[string]tftypes.Value) map[string]tftypes.Value {
	newVals := map[string]tftypes.Value{}
	for key, value := range values {
		newVals[key] = value
	}
	return newVals
}

func terraform5Type(values map[string]tftypes.Value) tftypes.Type {
	types := map[string]tftypes.Type{}
	for name, val := range values {
		types[name] = val.Type()
	}

	return tftypes.Object{AttributeTypes: types}
}

func terraform5Value(values map[string]tftypes.Value) tftypes.Value {
	return tftypes.NewValue(terraform5Type(values), values)
}

func applyDefaults(defaults map[string]TFType, attributes map[string]TFType) error {
	for name, defaultValue := range defaults {
		if attribute, ok := attributes[name]; ok {
			attributeHasValue := !attribute.TFValue().IsNull() && attribute.TFValue().IsKnown()
			defaultHasValue := defaultValue.TFValue().IsKnown() && !defaultValue.TFValue().IsNull()
			if !attributeHasValue && defaultHasValue {
				if err := attribute.FromTFValue(defaultValue.TFValue()); err != nil {
					return fmt.Errorf("failed to apply default to attribute: %s", name)
				}
			}
		} else {
			return fmt.Errorf("failed to apply default to attribute: [%s], since it is missing", name)
		}
	}
	return nil
}

// isTransportConfigured checks if the provided transport is configured. A transport is considered
// configured, if at least one of its attributes has a value. It is not sufficient to just check if
// the length of the Values array is > 0 since that only means that the transport received some of
// its configuration from the resources transport stanza. In some cases a transport can be entirely
// configured via the default transport defined in the provider's transport stanza. In this case the
// attributes will have values, and the Values array will be empty.
func isTransportConfigured(state transportState) bool {
	for _, attribute := range state.Attributes() {
		if !attribute.TFValue().IsNull() {
			return true
		}
	}
	return false
}

// checkK8STransportNotConfigured verifies that the Kubernetes transport is not configured. An error
// is returned if the K8S transport is configured.
func checkK8STransportNotConfigured(state StateWithTransport, resourceName string) error {
	if state.EmbeddedTransport().K8S.IsConfigured() {
		return newErrWithDiagnostics("invalid configuration", fmt.Sprintf("the '%s' resource does not support the 'kubernetes' transport", resourceName))
	}
	return nil
}
