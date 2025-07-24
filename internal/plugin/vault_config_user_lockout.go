// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type vaultUserLockoutConfig struct {
	LockoutThreshold    *tfString
	LockoutDuration     *tfString
	LockoutCounterReset *tfString
	DisableLockout      *tfBool

	RawValues map[string]tftypes.Value
	RawValue  tftypes.Value
	Unknown   bool
	Null      bool
}

func newVaultUserLockoutConfig() *vaultUserLockoutConfig {
	return &vaultUserLockoutConfig{
		LockoutThreshold:    newTfString(),
		LockoutDuration:     newTfString(),
		LockoutCounterReset: newTfString(),
		DisableLockout:      newTfBool(),
		Unknown:             false,
		Null:                true,
	}
}

func (s *vaultUserLockoutConfig) FromTerraform5Value(val tftypes.Value) error {
	if s == nil {
		return fmt.Errorf("cannot unmarshal %s into nil vaultUserLockoutConfig", val.String())
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
	if err := val.As(&s.RawValues); err != nil {
		return err
	}
	// Decode known fields
	for k, v := range s.RawValues {
		switch k {
		case "lockout_threshold":
			if err := s.LockoutThreshold.FromTFValue(v); err != nil {
				return err
			}
		case "lockout_duration":
			if err := s.LockoutDuration.FromTFValue(v); err != nil {
				return err
			}
		case "lockout_counter_reset":
			if err := s.LockoutCounterReset.FromTFValue(v); err != nil {
				return err
			}
		case "disable_lockout":
			if err := s.DisableLockout.FromTFValue(v); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown user_lockout config key: %s", k)
		}
	}
	return nil
}

func (s *vaultUserLockoutConfig) Terraform5Type() tftypes.Type {
	return tftypes.DynamicPseudoType
}

func (s *vaultUserLockoutConfig) Terraform5Value() tftypes.Value {
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
		case "lockout_threshold":
			vals[name] = s.LockoutThreshold.TFValue()
		case "lockout_duration":
			vals[name] = s.LockoutDuration.TFValue()
		case "lockout_counter_reset":
			vals[name] = s.LockoutCounterReset.TFValue()
		case "disable_lockout":
			vals[name] = s.DisableLockout.TFValue()
		}
		attrs[name] = vals[name].Type()
	}
	if len(vals) == 0 {
		return tftypes.NewValue(s.Terraform5Type(), nil)
	}
	if s.RawValue.Type().Is(tftypes.Map{}) {
		for _, v := range vals {
			return tftypes.NewValue(tftypes.Map{ElementType: v.Type()}, vals)
		}
	}
	return tftypes.NewValue(tftypes.Object{AttributeTypes: attrs}, vals)
}
