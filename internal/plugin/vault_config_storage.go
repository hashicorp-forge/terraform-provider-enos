// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type vaultStorageConfig struct {
	Type *tfString

	Attrs     *dynamicPseudoTypeBlock
	RetryJoin *dynamicPseudoTypeBlock

	// keep these around for marshaling the dynamic value
	RawValues map[string]tftypes.Value
	RawValue  tftypes.Value

	Unknown bool
	Null    bool
}

type vaultStorageConfigSet struct {
	typ       string
	attrs     map[string]any
	retryJoin map[string]any
}

func newVaultStorageConfig() *vaultStorageConfig {
	return &vaultStorageConfig{
		Type:      newTfString(),
		Attrs:     newDynamicPseudoTypeBlock(),
		RetryJoin: newDynamicPseudoTypeBlock(),
		Unknown:   false,
		Null:      true,
	}
}

func newVaultStorageConfigSet(typ string, attrs map[string]any, retryJoin map[string]any) *vaultStorageConfigSet {
	return &vaultStorageConfigSet{
		typ:       typ,
		attrs:     attrs,
		retryJoin: retryJoin,
	}
}

func (s *vaultStorageConfig) Set(set *vaultStorageConfigSet) {
	if s == nil || set == nil {
		return
	}

	s.Unknown = false
	s.Type.Set(set.typ)
	s.Attrs.Object.Set(set.attrs)
	s.RetryJoin.Object.Set(set.retryJoin)
}

// FromTerraform5Value unmarshals the value to the struct.
func (s *vaultStorageConfig) FromTerraform5Value(val tftypes.Value) error {
	if s == nil {
		return AttributePathError(fmt.Errorf("cannot unmarshal %s into nil vault storage config", val.String()),
			"config", "storage",
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

	// Since attributes is a dynamic pseudo type we have to decode it only
	// if it's known.
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
		case "retry_join":
			err = s.RetryJoin.FromTFValue(v)
			if err != nil {
				return err
			}
		default:
			return AttributePathError(fmt.Errorf("unknown configuration '%s', known values are 'type', 'attributes', or 'retry_join'", k),
				"config", "storage",
			)
		}
	}

	return nil
}

// Terraform5Type is the tftypes.Type.
func (s *vaultStorageConfig) Terraform5Type() tftypes.Type {
	return tftypes.DynamicPseudoType
}

// Terraform5Value is the tftypes.Value.
func (s *vaultStorageConfig) Terraform5Value() tftypes.Value {
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
		case "retry_join":
			val, err = s.RetryJoin.TFValue()
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
