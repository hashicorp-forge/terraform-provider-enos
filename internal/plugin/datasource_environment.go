package plugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/enos-provider/internal/diags"
	"github.com/hashicorp/enos-provider/internal/server/datarouter"
	"github.com/hashicorp/enos-provider/internal/server/state"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
		err = errors.Join(err, fmt.Errorf("unable to resolve public ip address"))
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
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:     "id",
					Type:     tftypes.String,
					Computed: true,
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
					Computed: true,
				},
				{
					Name: "public_ipv4_addresses",
					Type: tftypes.List{
						ElementType: tftypes.String,
					},
					Computed: true,
				},
				{
					Name: "public_ipv6_addresses",
					Type: tftypes.List{
						ElementType: tftypes.String,
					},
					Computed: true,
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
	_, err := mapAttributesTo(val, map[string]interface{}{
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
