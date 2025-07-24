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
	vals := map[string]tftypes.Value{}
	err := val.As(&vals)
	if err != nil {
		return err
	}
	for k, v := range vals {
		switch k {
		case "lockout_threshold":
			err = s.LockoutThreshold.FromTFValue(v)
		case "lockout_duration":
			err = s.LockoutDuration.FromTFValue(v)
		case "lockout_counter_reset":
			err = s.LockoutCounterReset.FromTFValue(v)
		case "disable_lockout":
			err = s.DisableLockout.FromTFValue(v)
		default:
			return fmt.Errorf("unknown user_lockout config key: %s", k)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *vaultUserLockoutConfig) Terraform5Type() tftypes.Type {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"lockout_threshold":     tftypes.String,
			"lockout_duration":      tftypes.String,
			"lockout_counter_reset": tftypes.String,
			"disable_lockout":       tftypes.Bool,
		},
		OptionalAttributes: map[string]struct{}{
			"lockout_threshold":     {},
			"lockout_duration":      {},
			"lockout_counter_reset": {},
			"disable_lockout":       {},
		},
	}
}

func (s *vaultUserLockoutConfig) Terraform5Value() tftypes.Value {
	if s.Null {
		return tftypes.NewValue(s.Terraform5Type(), nil)
	}
	if s.Unknown {
		return tftypes.NewValue(s.Terraform5Type(), tftypes.UnknownValue)
	}
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"lockout_threshold":     s.LockoutThreshold.TFValue(),
		"lockout_duration":      s.LockoutDuration.TFValue(),
		"lockout_counter_reset": s.LockoutCounterReset.TFValue(),
		"disable_lockout":       s.DisableLockout.TFValue(),
	})
}
