// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type vaultStorageConfig struct {
	Type *tfString

	Attrs       *tfObject
	AttrsRaw    tftypes.Value
	AttrsValues map[string]tftypes.Value

	RetryJoin       *tfObject
	RetryJoinRaw    tftypes.Value
	RetryJoinValues map[string]tftypes.Value

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
		Attrs:     newTfObject(),
		RetryJoin: newTfObject(),
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
	s.Attrs.Set(set.attrs)
	s.RetryJoin.Set(set.retryJoin)
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
			if !v.IsKnown() {
				// Attrs are a DynamicPseudoType but the value is unknown. Terraform expects us to be a
				// dynamic value that we'll know after apply.
				s.Attrs.Unknown = true
				continue
			}
			if v.IsNull() {
				// We can't unmarshal null or unknown things
				continue
			}

			s.AttrsRaw = v
			err = v.As(&s.AttrsValues)
			if err != nil {
				return err
			}
			err = s.Attrs.FromTFValue(v)
			if err != nil {
				return err
			}
		case "retry_join":
			if !v.IsKnown() {
				// RetryJoin are a DynamicPseudoType but the value is unknown. Terraform expects us to be a
				// dynamic value that we'll know after apply.
				s.RetryJoin.Unknown = true
				continue
			}
			if v.IsNull() {
				// We can't unmarshal null or unknown things
				continue
			}

			s.RetryJoinRaw = v
			err = v.As(&s.RetryJoinValues)
			if err != nil {
				return err
			}
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
		switch name {
		case "type":
			attrs[name] = s.Type.TFType()
			vals[name] = s.Type.TFValue()
		case "attributes":
			var attrsVal tftypes.Value
			if s.AttrsRaw.Type() == nil {
				// We don't have a type, which means we're a DynamicPseudoType with either a nil or unknown
				// value.
				if s.Attrs.Unknown {
					attrsVal = tftypes.NewValue(tftypes.DynamicPseudoType, tftypes.UnknownValue)
				} else {
					attrsVal = tftypes.NewValue(tftypes.DynamicPseudoType, nil)
				}
			} else {
				var err error
				attrsVal, err = encodeTfObjectDynamicPseudoType(s.AttrsRaw, s.AttrsValues)
				if err != nil {
					panic(err)
				}
			}
			attrs[name] = attrsVal.Type()
			vals[name] = attrsVal
		case "retry_join":
			var retryVal tftypes.Value
			if s.RetryJoinRaw.Type() == nil {
				// We don't have a type, which means we're a DynamicPseudoType with either a nil or unknown
				// value.
				if s.RetryJoin.Unknown {
					retryVal = tftypes.NewValue(tftypes.DynamicPseudoType, tftypes.UnknownValue)
				} else {
					retryVal = tftypes.NewValue(tftypes.DynamicPseudoType, nil)
				}
			} else {
				var err error
				retryVal, err = encodeTfObjectDynamicPseudoType(s.RetryJoinRaw, s.RetryJoinValues)
				if err != nil {
					panic(err)
				}
			}
			attrs[name] = retryVal.Type()
			vals[name] = retryVal
		default:
		}
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
