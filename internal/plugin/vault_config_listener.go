// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type vaultListenerConfig struct {
	Type *tfString

	// Top-level listener attributes
	Attrs *dynamicPseudoTypeBlock

	// Sub-stanzas in 'listener'
	Telemetry *dynamicPseudoTypeBlock
	Profiling *dynamicPseudoTypeBlock
	// inflight_requests_logging stanza
	IRL *dynamicPseudoTypeBlock
	// custom_response_headers stanza
	CRH *dynamicPseudoTypeBlock

	// Keep the raw values around for marshaling the dynamic value
	RawValues map[string]tftypes.Value
	RawValue  tftypes.Value

	Unknown bool
	Null    bool
}

type vaultListenerConfigSet struct {
	typ   string
	attrs map[string]map[string]any
}

func newVaultListenerConfig() *vaultListenerConfig {
	return &vaultListenerConfig{
		Type:      newTfString(),
		Attrs:     newDynamicPseudoTypeBlock(),
		Telemetry: newDynamicPseudoTypeBlock(),
		Profiling: newDynamicPseudoTypeBlock(),
		IRL:       newDynamicPseudoTypeBlock(),
		CRH:       newDynamicPseudoTypeBlock(),
		Unknown:   false,
		Null:      true,
	}
}

func newVaultListenerConfigSet(typ string, attrs map[string]map[string]any) *vaultListenerConfigSet {
	return &vaultListenerConfigSet{typ: typ, attrs: attrs}
}

func (s *vaultListenerConfig) Set(set *vaultListenerConfigSet) {
	if s == nil || set == nil {
		return
	}

	s.Unknown = false
	s.Type.Set(set.typ)

	for name, values := range set.attrs {
		switch name {
		case "attributes":
			s.Attrs.Object.Set(values)
		case "telemetry":
			s.Telemetry.Object.Set(values)
		case "profiling":
			s.Profiling.Object.Set(values)
		case "inflight_requests_logging":
			s.IRL.Object.Set(values)
		case "custom_response_headers":
			s.CRH.Object.Set(values)
		}
	}
}

// FromTerraform5Value unmarshals the value to the struct.
func (s *vaultListenerConfig) FromTerraform5Value(val tftypes.Value) error {
	if s == nil {
		return AttributePathError(fmt.Errorf("cannot unmarshal %s into nil vault listener config", val.String()),
			"config", "listener",
		)
	}

	if val.IsNull() {
		s.Null = true
		s.Unknown = false

		return nil
	}

	if !val.IsKnown() {
		s.Unknown = true

		return nil
	}

	s.Null = false
	s.Unknown = false
	s.RawValue = val
	s.RawValues = map[string]tftypes.Value{}
	err := val.As(&s.RawValues)
	if err != nil {
		return err
	}

	// Decode each nested attribute
	for k, v := range s.RawValues {
		switch k {
		case "type":
			err = s.Type.FromTFValue(v)
			if err != nil {
				return err
			}
		case "attributes":
			err = s.Attrs.FromTFValue(v)
			if err != nil {
				return err
			}
		case "telemetry":
			err = s.Telemetry.FromTFValue(v)
			if err != nil {
				return err
			}
		case "profiling":
			err = s.Profiling.FromTFValue(v)
			if err != nil {
				return err
			}
		case "inflight_requests_logging":
			err = s.IRL.FromTFValue(v)
			if err != nil {
				return err
			}
		case "custom_response_headers":
			err = s.CRH.FromTFValue(v)
			if err != nil {
				return err
			}
		default:
			return AttributePathError(fmt.Errorf("unknown configuration '%s'", k), "config", "listener")
		}
	}

	return nil
}

// Terraform5Type is the tftypes.Type.
func (s *vaultListenerConfig) Terraform5Type() tftypes.Type {
	return tftypes.DynamicPseudoType
}

// Terraform5Value is the tftypes.Value.
func (s *vaultListenerConfig) Terraform5Value() tftypes.Value {
	if s.Null {
		return tftypes.NewValue(s.Terraform5Type(), nil)
	}

	if s.Unknown {
		return tftypes.NewValue(s.Terraform5Type(), tftypes.UnknownValue)
	}

	attrs := map[string]tftypes.Type{}
	vals := map[string]tftypes.Value{}
	for name := range s.RawValues {
		var val tftypes.Value
		var err error
		switch name {
		case "type":
			val = s.Type.TFValue()
		case "attributes":
			val, err = s.Attrs.TFValue()
			if err != nil {
				panic(err)
			}
		case "telemetry":
			val, err = s.Telemetry.TFValue()
			if err != nil {
				panic(err)
			}
		case "profiling":
			val, err = s.Profiling.TFValue()
			if err != nil {
				panic(err)
			}
		case "inflight_requests_logging":
			val, err = s.IRL.TFValue()
			if err != nil {
				panic(err)
			}
		case "custom_response_headers":
			val, err = s.CRH.TFValue()
			if err != nil {
				panic(err)
			}
		default:
		}

		attrs[name] = val.Type()
		vals[name] = val
	}

	if len(vals) == 0 {
		return tftypes.NewValue(tftypes.DynamicPseudoType, nil)
	}

	// Depending on how many are set, Terraform might pass the configuration over
	// as a map or object, so we need to handle both.
	if s.RawValue.Type().Is(tftypes.Map{}) {
		for _, val := range vals {
			return tftypes.NewValue(tftypes.Map{ElementType: val.Type()}, vals)
		}
	}

	return tftypes.NewValue(tftypes.Object{AttributeTypes: attrs}, vals)
}
