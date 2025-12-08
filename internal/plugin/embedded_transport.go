// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"text/template"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/state"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
)

// transportClientFactory Factory function for creating transport clients, can be overridden in tests.
type transportClientFactory = func(ctx context.Context, transport transportState) (it.Transport, error)

var defaultTransportClientFactory = func(ctx context.Context, transport transportState) (it.Transport, error) {
	return transport.Client(ctx)
}

const (
	UNKNOWN it.TransportType = "unknown"
	SSH     it.TransportType = "ssh"
	K8S     it.TransportType = "kubernetes"
	NOMAD   it.TransportType = "nomad"
)

func createTransport(t it.TransportType) (transportState, error) {
	switch t {
	case SSH:
		return newEmbeddedTransportSSH(), nil
	case K8S:
		return newEmbeddedTransportK8Sv1(), nil
	case NOMAD:
		return newEmbeddedTransportNomadv1(), nil
	case UNKNOWN:
		return nil, errors.New("cannot create a UNKNOWN transport")
	default:
		return nil, errors.New("cannot create an undefined transport")
	}
}

func isKnownTransportType(t it.TransportType) bool {
	switch t {
	case SSH, K8S, NOMAD:
		return true
	case UNKNOWN:
		return false
	default:
		return false
	}
}

func transportTypeFrom(typeString string) it.TransportType {
	switch typeString {
	case "ssh":
		return SSH
	case "kubernetes":
		return K8S
	case "nomad":
		return NOMAD
	}

	return UNKNOWN
}

var TransportTypes = []it.TransportType{SSH, K8S, NOMAD}

type Transports map[it.TransportType]transportState

func (t Transports) types() []it.TransportType {
	types := make([]it.TransportType, len(t))

	count := 0
	for tType := range t {
		types[count] = tType
		count++
	}

	return types
}

// getConfiguredTransport gets the configured transport if there is only one configured. If there
// is more than one configured transport or none the second parameter will be false and an error
// is returned. The error is returned in addition to the bool value since it will contain the
// appropriate error message.
func (t Transports) getConfiguredTransport() (transportState, bool, error) {
	var transport transportState
	for _, trans := range t {
		if transport != nil {
			return nil, false,
				fmt.Errorf("invalid transport configuration, only one transport can be configured, %v were configured", t.types())
		}
		transport = trans
	}

	if transport == nil {
		return nil, false,
			fmt.Errorf("invalid transport configuration, no transport configured, one of %v must be configured", TransportTypes)
	}

	return transport, true, nil
}

var transportTmpl = template.Must(template.New("transport").Parse(`
transport = {
  {{range $config := .}}
  {{$config}}
  {{end}}
}`))

// embeddedTransportV1 represents the embedded transport state for all
// resources and data source. It is intended to be used as output from the
// transport data source and used as the transport input for all resources.
type embeddedTransportV1 struct {
	mu            sync.Mutex
	transports    Transports
	clientFactory transportClientFactory

	// resolvedTransport the transport state that was resolved after applying defaults from the
	// provider configuration
	resolvedTransport transportState
}

// transportState interface defining the api of a transport.
type transportState interface {
	state.Serializable

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

	Type() it.TransportType

	// render renders the transport state to HCL
	render() (string, error)

	// debug exports the state as a string in a format suitable for logging as part of a diagnostic
	// message
	debug() string
}

func newEmbeddedTransport() *embeddedTransportV1 {
	return &embeddedTransportV1{
		mu:            sync.Mutex{},
		transports:    map[it.TransportType]transportState{},
		clientFactory: defaultTransportClientFactory,
	}
}

func (em *embeddedTransportV1) SSH() (*embeddedTransportSSHv1, bool) {
	transport, ok := em.transports[SSH]
	if !ok {
		return nil, false
	}
	ssh, ok := transport.(*embeddedTransportSSHv1)
	if !ok {
		return nil, false
	}

	return ssh, true
}

func (em *embeddedTransportV1) K8S() (*embeddedTransportK8Sv1, bool) {
	transport, ok := em.transports[K8S]
	if !ok {
		return nil, false
	}
	k8s, ok := transport.(*embeddedTransportK8Sv1)
	if !ok {
		return nil, false
	}

	return k8s, true
}

func (em *embeddedTransportV1) Nomad() (*embeddedTransportNomadv1, bool) {
	transport, ok := em.transports[NOMAD]
	if !ok {
		return nil, false
	}
	nomad, ok := transport.(*embeddedTransportNomadv1)
	if !ok {
		return nil, false
	}

	return nomad, true
}

func (em *embeddedTransportV1) SetTransportState(states ...transportState) error {
	for _, state := range states {
		transportType := state.Type()
		if transportType == UNKNOWN {
			return errors.New("failed to set transport state for unknown transport type: ")
		}
		em.transports[transportType] = state
	}

	return nil
}

// Allow resources to specify which transports are supported when declaring their schema.
type supportedTransports int

const (
	supportsSSH supportedTransports = 1 << iota
	supportsK8s
	supportsNomad
)

// SchemaAttributeTransport is our transport schema configuration attribute.
// Resources that embed a transport should use this as transport schema.
func (em *embeddedTransportV1) SchemaAttributeTransport(supports supportedTransports) *tfprotov6.SchemaAttribute {
	b := strings.Builder{}

	if supports&supportsSSH == supportsSSH {
		b.WriteString(sshTransportSchemaMarkdown)
	}
	if supports&supportsK8s == supportsK8s {
		b.WriteString(k8sTransportSchemaMarkdown)
	}
	if supports&supportsNomad == supportsNomad {
		b.WriteString(nomadTransportSchemaMarkdown)
	}

	return &tfprotov6.SchemaAttribute{
		Name:            "transport",
		Type:            em.Terraform5Type(),
		DescriptionKind: tfprotov6.StringKindMarkdown,
		Description:     b.String(),
		Optional:        true, // We'll handle our own schema validation
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

	for key, val := range vals {
		tType := transportTypeFrom(key)
		if !isKnownTransportType(tType) {
			return fmt.Errorf("failed to unmarshall unknown transport config: %s", key)
		}
		transport, err := createTransport(tType)
		if err != nil {
			return err
		}
		if val.IsKnown() {
			if err := transport.FromTerraform5Value(val); err != nil {
				return err
			}
		}
		em.transports[tType] = transport
	}

	return nil
}

// Terraform5Type is the tftypes.Type.
func (em *embeddedTransportV1) Terraform5Type() tftypes.Type {
	return tftypes.DynamicPseudoType
}

// Terraform5Value generates a tftypes.Value. This must return a value that is structurally the same as
// the value unmarshalled via FromTerraform5Value. Do not try to add or remove attributes, Terraform
// does not like that.
func (em *embeddedTransportV1) Terraform5Value() tftypes.Value {
	if len(em.transports) == 0 {
		return tftypes.NewValue(em.Terraform5Type(), nil)
	}

	values := map[string]tftypes.Value{}
	attributeTypes := map[string]tftypes.Type{}
	for tType, transport := range em.transports {
		value := transport.Terraform5Value()
		values[string(tType)] = value
		attributeTypes[string(tType)] = value.Type()
	}

	return tftypes.NewValue(tftypes.Object{AttributeTypes: attributeTypes}, values)
}

// Copy makes an identical copy of the transport. The copy will not share any of the data structures
// of this transport and is safe to modify.
func (em *embeddedTransportV1) Copy() (*embeddedTransportV1, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	factory := em.clientFactory

	newCopy := newEmbeddedTransport()
	newCopy.clientFactory = factory

	if err := newCopy.FromTerraform5Value(em.Terraform5Value()); err != nil {
		return nil, err
	}

	return newCopy, nil
}

// CopyValues returns only the values that have received their configuration from the
// terraform configuration for that resource. The configured attributes do not include the attributes
// that may have been received as default values via ApplyDefaults. To get all the attribute values
// including those received via defaults use the Attributes method instead.
func (em *embeddedTransportV1) CopyValues() map[it.TransportType]map[string]tftypes.Value {
	configuredAttributes := map[it.TransportType]map[string]tftypes.Value{}

	for tType, transport := range em.transports {
		configuredAttributes[tType] = transport.CopyValues()
	}

	return configuredAttributes
}

// Attributes returns all the attributes of the transport state. This includes those that where
// configured by the user (via terraform configuration) and those that received their values
// via defaults.
func (em *embeddedTransportV1) Attributes() map[it.TransportType]map[string]TFType {
	attributes := map[it.TransportType]map[string]TFType{}

	for tType, transport := range em.transports {
		attributes[tType] = transport.Attributes()
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

	return em.clientFactory(ctx, transport)
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
func (em *embeddedTransportV1) ApplyDefaults(defaults *embeddedTransportV1) (transportState, error) {
	defaults.mu.Lock()
	defer defaults.mu.Unlock()
	em.mu.Lock()
	defer em.mu.Unlock()

	configuredCount := len(em.transports)
	if configuredCount == 1 {
		if configured, configuredOk, _ := em.transports.getConfiguredTransport(); configuredOk {
			if defaultsTransport, defaultsOk := defaults.transports[configured.Type()]; defaultsOk {
				err := configured.ApplyDefaults(defaultsTransport.Attributes())
				return configured, err
			}

			return configured, nil
		}
	}

	// If this embedded transport configuration does not define a transport block, then it should
	// receive its transport configuration entirely from the enos provider configuration.
	// In this case the transport defaults cannot be configured with more than one  transport since
	// it would not be possible to know which transport client to build when applying the resource.
	if configuredCount == 0 {
		defaultsTransport, err := defaults.GetConfiguredTransport()
		if err != nil {
			return nil, err
		}

		transport, err := createTransport(defaultsTransport.Type())
		if err != nil {
			return nil, err
		}
		if err := transport.ApplyDefaults(defaultsTransport.Attributes()); err != nil {
			return nil, err
		}
		if err := em.SetTransportState(transport); err != nil {
			return nil, err
		}

		return transport, nil
	}

	return nil, fmt.Errorf(
		"invalid transport configuration, only one transport can be configured, %v were configured",
		em.transports.types(),
	)
}

// GetConfiguredTransport Gets the configured transport for this embedded transport. There should be
// only one configured transport for a resource. If there are none configured or more than one configured
// calling this method will return an error.
func (em *embeddedTransportV1) GetConfiguredTransport() (transportState, error) {
	transport, _, err := em.transports.getConfiguredTransport()
	if err != nil {
		return nil, err
	}

	return transport, nil
}

func (em *embeddedTransportV1) Debug() string {
	if em.resolvedTransport != nil {
		return em.resolvedTransport.debug() + "\n"
	}

	debug := make([]string, len(em.transports))
	c := 0
	for _, t := range em.transports {
		debug[c] = t.debug() + "\n"
		c++
	}

	return strings.Join(debug, "\n\n")
}

func (em *embeddedTransportV1) transportReplacedAttributePaths(proposed *embeddedTransportV1) []*tftypes.AttributePath {
	var attrs []*tftypes.AttributePath

	for tType, transport := range em.transports {
		proposedTransport := proposed.transports[tType]
		if transport.IsConfigured() && proposedTransport.IsConfigured() {
			addAttributesForReplace(transport, proposedTransport, attrs, string(tType))
		}
	}

	if len(attrs) > 0 {
		return attrs
	}

	return nil
}

func (em *embeddedTransportV1) render() (string, error) {
	if len(em.transports) == 0 {
		return "", nil
	}

	cfgs := make([]string, len(em.transports))
	count := 0
	for i := range em.transports {
		cfg, err := em.transports[i].render()
		if err != nil {
			return "", err
		}
		cfgs[count] = cfg
		count++
	}

	buf := bytes.Buffer{}
	if err := transportTmpl.Execute(&buf, cfgs); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (em *embeddedTransportV1) setResolvedTransport(transport transportState) {
	em.resolvedTransport = transport
}

func addAttributesForReplace(priorState, proposedState transportState, attrs []*tftypes.AttributePath, transportName string) []*tftypes.AttributePath {
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

func verifyConfiguration(knownAttributes []string, values map[string]tftypes.Value, transportType string) error {
	// If we're verifying the configuration we must have some values. This error indicates that
	// someone configured the transport with null or an empty map/object which is not allowed, i.e.:
	// transport = { kubernetes = {} } or transport = { nomad = null }
	if len(values) == 0 {
		return fmt.Errorf("%s transport configuration cannot be empty", transportType)
	}
	// Because the transport type is a dynamic pseudo type we have to manually ensure
	// that the user hasn't set any unknown attributes.
	isKnownAttribute := func(attr string) error {
		if slices.Contains(knownAttributes, attr) {
			return nil
		}

		return AttributePathError(
			fmt.Errorf("unsupported argument, an argument named \"%s\" is not expected here", attr),
			"transport", transportType, attr)
	}

	for attribute := range values {
		err := isKnownAttribute(attribute)
		if err != nil {
			return err
		}
	}

	return nil
}

func copyValues(values map[string]tftypes.Value) map[string]tftypes.Value {
	newVals := map[string]tftypes.Value{}
	maps.Copy(newVals, values)

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

func applyDefaults(defaults, attributes map[string]TFType) error {
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
	if _, ok := state.EmbeddedTransport().transports[K8S]; ok {
		return fmt.Errorf(
			"invalid configuration, the '%s' resource does not support the 'kubernetes' transport",
			resourceName,
		)
	}

	return nil
}
