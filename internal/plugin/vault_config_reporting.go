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
}

func newVaultReportingLicenseConfig() *vaultReportingLicenseConfig {
	return &vaultReportingLicenseConfig{
		Enabled:               newTfBool(),
		BillingStartTimestamp: newTfString(),
		DevelopmentCluster:    newTfBool(),
	}
}

type vaultReportingConfig struct {
	SnapshotRetentionTime        *tfString
	DisableProductUsageReporting *tfBool
	License                      *vaultReportingLicenseConfig
	Unknown                      bool
	Null                         bool
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
	vals := map[string]tftypes.Value{}
	err := val.As(&vals)
	if err != nil {
		return err
	}
	for k, v := range vals {
		switch k {
		case "snapshot_retention_time":
			err = s.SnapshotRetentionTime.FromTFValue(v)
		case "disable_product_usage_reporting":
			err = s.DisableProductUsageReporting.FromTFValue(v)
		case "license":
			err = s.License.FromTerraform5Value(v)
		default:
			return fmt.Errorf("unknown reporting config key: %s", k)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *vaultReportingConfig) Terraform5Type() tftypes.Type {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"snapshot_retention_time":         tftypes.String,
			"disable_product_usage_reporting": tftypes.Bool,
			"license":                         newVaultReportingLicenseConfig().Terraform5Type(),
		},
		OptionalAttributes: map[string]struct{}{
			"snapshot_retention_time":         {},
			"disable_product_usage_reporting": {},
			"license":                         {},
		},
	}
}

func (s *vaultReportingConfig) Terraform5Value() tftypes.Value {
	if s.Null {
		return tftypes.NewValue(s.Terraform5Type(), nil)
	}
	if s.Unknown {
		return tftypes.NewValue(s.Terraform5Type(), tftypes.UnknownValue)
	}
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"snapshot_retention_time":         s.SnapshotRetentionTime.TFValue(),
		"disable_product_usage_reporting": s.DisableProductUsageReporting.TFValue(),
		"license":                         s.License.Terraform5Value(),
	})
}

func (s *vaultReportingLicenseConfig) FromTerraform5Value(val tftypes.Value) error {
	if s == nil {
		return fmt.Errorf("cannot unmarshal %s into nil vaultReportingLicenseConfig", val.String())
	}
	if val.IsNull() {
		return nil
	}
	if !val.IsKnown() {
		return nil
	}
	vals := map[string]tftypes.Value{}
	err := val.As(&vals)
	if err != nil {
		return err
	}
	for k, v := range vals {
		switch k {
		case "enabled":
			err = s.Enabled.FromTFValue(v)
		case "billing_start_timestamp":
			err = s.BillingStartTimestamp.FromTFValue(v)
		case "development_cluster":
			err = s.DevelopmentCluster.FromTFValue(v)
		default:
			return fmt.Errorf("unknown license config key: %s", k)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *vaultReportingLicenseConfig) Terraform5Type() tftypes.Type {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"enabled":                 tftypes.Bool,
			"billing_start_timestamp": tftypes.String,
			"development_cluster":     tftypes.Bool,
		},
		OptionalAttributes: map[string]struct{}{
			"enabled":                 {},
			"billing_start_timestamp": {},
			"development_cluster":     {},
		},
	}
}

func (s *vaultReportingLicenseConfig) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"enabled":                 s.Enabled.TFValue(),
		"billing_start_timestamp": s.BillingStartTimestamp.TFValue(),
		"development_cluster":     s.DevelopmentCluster.TFValue(),
	})
}
