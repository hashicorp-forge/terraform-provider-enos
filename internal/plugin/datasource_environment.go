// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/diags"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/datarouter"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/state"
)

type environment struct {
	providerConfig *config
}

var _ datarouter.DataSource = (*environment)(nil)

type environmentStateV1 struct {
	ID                  *tfString
	PublicIPAddress     *tfString
	PublicIPAddresses   *tfStringSlice
	PublicIPV4Addresses *tfStringSlice
	PublicIPV6Addresses *tfStringSlice

	failureHandlers
}

var _ state.State = (*environmentStateV1)(nil)

func newEnvironment() *environment {
	return &environment{
		providerConfig: newProviderConfig(),
	}
}

func newEnvironmentStateV1() *environmentStateV1 {
	return &environmentStateV1{
		ID:                  newTfString(),
		PublicIPAddress:     newTfString(),
		PublicIPAddresses:   newTfStringSlice(),
		PublicIPV4Addresses: newTfStringSlice(),
		PublicIPV6Addresses: newTfStringSlice(),
		failureHandlers:     failureHandlers{},
	}
}

func (d *environment) Name() string {
	return "enos_environment"
}

func (d *environment) Schema() *tfprotov6.Schema {
	return newEnvironmentStateV1().Schema()
}

func (d *environment) SetProviderConfig(meta tftypes.Value) error {
	return d.providerConfig.FromTerraform5Value(meta)
}

// ValidateDataResourceConfig is the request Terraform sends when it wants to
// validate the data source's configuration.
func (d *environment) ValidateDataResourceConfig(ctx context.Context, req tfprotov6.ValidateDataResourceConfigRequest, res *tfprotov6.ValidateDataResourceConfigResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	// unmarshal it to our known type to ensure whatever was passed in matches
	// the correct schema.
	newConfig := newEnvironmentStateV1()
	err := unmarshal(newConfig, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
	}
}

// ReadDataSource is the request Terraform sends when it wants to get the latest
// state for the data source.
func (d *environment) ReadDataSource(ctx context.Context, req tfprotov6.ReadDataSourceRequest, res *tfprotov6.ReadDataSourceResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	newState := newEnvironmentStateV1()

	// unmarshal and re-marshal the state to add default fields
	err := unmarshal(newState, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}
	newState.ID.Set("static")

	resolver := newPublicIPResolver()
	err = resolver.resolve(ctx, defaultResolvers()...)
	if len(resolver.ips()) == 0 {
		err = errors.Join(err, errors.New("unable to resolve public ip address"))
	}
	if err != nil {
		res.Diagnostics = append(res.Diagnostics,
			diags.ErrToDiagnostic("Resolve IP Error", fmt.Errorf("failed to resolve public IP addresses, due to: %w", err)),
		)

		return
	}

	newState.PublicIPAddress.Set(resolver.ipsStrings()[0])
	newState.PublicIPAddresses.SetStrings(resolver.ipsStrings())
	newState.PublicIPV4Addresses.SetStrings(resolver.v4Strings())
	newState.PublicIPV6Addresses.SetStrings(resolver.v6Strings())

	res.State, err = state.Marshal(newState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
	}
}

// Schema is the file states Terraform schema.
func (s *environmentStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			DescriptionKind: tfprotov6.StringKindMarkdown,
			Description: docCaretToBacktick(`The ^enos_environment^ datasource is a datasource that we can use to pass environment specific
information into our Terraform run. As enos relies on SSH to execute the bulk of its actions, a
common problem is granting access to the host executing the Terraform run. As such, the
enos_environment resource can be used to determine our external IP addresses so that we can dynamically
generate security groups that allow only access from our end.
`),
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:        "id",
					Type:        tftypes.String,
					Computed:    true,
					Description: datasourceStaticIDDescription,
				},
				{
					Name:       "public_ip_address",
					Type:       tftypes.String,
					Computed:   true,
					Deprecated: true, // Use public_ip_addresses
				},
				{
					Name: "public_ip_addresses",
					Type: tftypes.List{
						ElementType: tftypes.String,
					},
					Computed:    true,
					Description: `All public IP addresses of the host executing Terraform. NOTE: can include both ipv4 and ipv6 addresses`,
				},
				{
					Name: "public_ipv4_addresses",
					Type: tftypes.List{
						ElementType: tftypes.String,
					},
					Computed:    true,
					Description: `The public IPv4 addresses of the host executing Terraform`,
				},
				{
					Name: "public_ipv6_addresses",
					Type: tftypes.List{
						ElementType: tftypes.String,
					},
					Computed:    true,
					Description: "The public IPv6 addresses of the host executing Terraform",
				},
			},
		},
	}
}

// Validate validates the configuration.
func (s *environmentStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *environmentStateV1) FromTerraform5Value(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]any{
		"id":                    s.ID,
		"public_ip_address":     s.PublicIPAddress,
		"public_ip_addresses":   s.PublicIPAddresses,
		"public_ipv4_addresses": s.PublicIPV4Addresses,
		"public_ipv6_addresses": s.PublicIPV6Addresses,
	})

	return err
}

// Terraform5Type is the file state tftypes.Type.
func (s *environmentStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":                    s.ID.TFType(),
		"public_ip_address":     s.PublicIPAddress.TFType(),
		"public_ip_addresses":   s.PublicIPAddresses.TFType(),
		"public_ipv4_addresses": s.PublicIPV4Addresses.TFType(),
		"public_ipv6_addresses": s.PublicIPV6Addresses.TFType(),
	}}
}

// Terraform5Value is the file state tftypes.Value.
func (s *environmentStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":                    s.ID.TFValue(),
		"public_ip_address":     s.PublicIPAddress.TFValue(),
		"public_ip_addresses":   s.PublicIPAddresses.TFValue(),
		"public_ipv4_addresses": s.PublicIPV4Addresses.TFValue(),
		"public_ipv6_addresses": s.PublicIPV6Addresses.TFValue(),
	})
}
