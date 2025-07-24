// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type vaultExperimentsConfig struct {
	Experiments []*tfString
	RawValues   map[string]tftypes.Value
	RawValue    tftypes.Value
	Unknown     bool
	Null        bool
}

func newVaultExperimentsConfig() *vaultExperimentsConfig {
	return &vaultExperimentsConfig{
		Experiments: []*tfString{},
		Unknown:     false,
		Null:        true,
	}
}

func (s *vaultExperimentsConfig) FromTerraform5Value(val tftypes.Value) error {
	if s == nil {
		return fmt.Errorf("cannot unmarshal %s into nil vaultExperimentsConfig", val.String())
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
	var experiments []string
	err := val.As(&experiments)
	if err != nil {
		return err
	}
	s.Experiments = []*tfString{}
	for _, exp := range experiments {
		ts := newTfString()
		ts.Set(exp)
		s.Experiments = append(s.Experiments, ts)
	}
	return nil
}

func (s *vaultExperimentsConfig) Terraform5Type() tftypes.Type {
	return tftypes.List{ElementType: tftypes.String}
}

func (s *vaultExperimentsConfig) Terraform5Value() tftypes.Value {
	if s.Null {
		return tftypes.NewValue(s.Terraform5Type(), nil)
	}
	if s.Unknown {
		return tftypes.NewValue(s.Terraform5Type(), tftypes.UnknownValue)
	}
	exps := []string{}
	for _, ts := range s.Experiments {
		if val, ok := ts.Get(); ok {
			exps = append(exps, val)
		}
	}
	return tftypes.NewValue(s.Terraform5Type(), exps)
}
