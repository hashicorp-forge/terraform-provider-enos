// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type vaultReportingLicenseConfig struct {
	Enabled               *tfBool
	BillingStartTimestamp *tfString
	DevelopmentCluster    *tfBool

	RawValues map[string]tftypes.Value
	RawValue  tftypes.Value
	Unknown   bool
	Null      bool
}

func newVaultReportingLicenseConfig() *vaultReportingLicenseConfig {
	return &vaultReportingLicenseConfig{
		Enabled:               newTfBool(),
		BillingStartTimestamp: newTfString(),
		DevelopmentCluster:    newTfBool(),
		Unknown:               false,
		Null:                  true,
	}
}

type vaultReportingConfig struct {
	SnapshotRetentionTime        *tfString
	DisableProductUsageReporting *tfBool
	License                      *vaultReportingLicenseConfig

	RawValues map[string]tftypes.Value
	RawValue  tftypes.Value
	Unknown   bool
	Null      bool
}

func newVaultReportingConfig() *vaultReportingConfig {
	return &vaultReportingConfig{
		SnapshotRetentionTime:        newTfString(),
		DisableProductUsageReporting: newTfBool(),
		License:                      newVaultReportingLicenseConfig(),
		Unknown:                      false,
		Null:                         true,
	}
}

func (s *vaultReportingConfig) FromTerraform5Value(val tftypes.Value) error {
	if s == nil {
		return fmt.Errorf("cannot unmarshal %s into nil vaultReportingConfig", val.String())
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
	for k, v := range s.RawValues {
		switch k {
		case "snapshot_retention_time":
			if err := s.SnapshotRetentionTime.FromTFValue(v); err != nil {
				return err
			}
		case "disable_product_usage_reporting":
			if err := s.DisableProductUsageReporting.FromTFValue(v); err != nil {
				return err
			}
		case "license":
			if err := s.License.FromTerraform5Value(v); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown reporting config key: %s", k)
		}
	}
	return nil
}

func (s *vaultReportingConfig) Terraform5Type() tftypes.Type {
	return tftypes.DynamicPseudoType
}

func (s *vaultReportingConfig) Terraform5Value() tftypes.Value {
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
		case "snapshot_retention_time":
			vals[name] = s.SnapshotRetentionTime.TFValue()
		case "disable_product_usage_reporting":
			vals[name] = s.DisableProductUsageReporting.TFValue()
		case "license":
			vals[name] = s.License.Terraform5Value()
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

func (s *vaultReportingLicenseConfig) FromTerraform5Value(val tftypes.Value) error {
	if s == nil {
		return fmt.Errorf("cannot unmarshal %s into nil vaultReportingLicenseConfig", val.String())
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
	for k, v := range s.RawValues {
		switch k {
		case "enabled":
			if err := s.Enabled.FromTFValue(v); err != nil {
				return err
			}
		case "billing_start_timestamp":
			if err := s.BillingStartTimestamp.FromTFValue(v); err != nil {
				return err
			}
		case "development_cluster":
			if err := s.DevelopmentCluster.FromTFValue(v); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown license config key: %s", k)
		}
	}
	return nil
}

func (s *vaultReportingLicenseConfig) Terraform5Type() tftypes.Type {
	return tftypes.DynamicPseudoType
}

func (s *vaultReportingLicenseConfig) Terraform5Value() tftypes.Value {
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
		case "enabled":
			vals[name] = s.Enabled.TFValue()
		case "billing_start_timestamp":
			vals[name] = s.BillingStartTimestamp.TFValue()
		case "development_cluster":
			vals[name] = s.DevelopmentCluster.TFValue()
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
