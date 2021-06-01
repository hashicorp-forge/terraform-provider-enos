package plugin

import (
	"context"
	"sync"

	"github.com/hashicorp/enos-provider/internal/server"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ server.Provider = (*Provider)(nil)
	_ Serializable    = (*config)(nil)
)

// newProvider returns a new instance of the plugin provider server
func newProvider() *Provider {
	return &Provider{
		mu:     sync.Mutex{},
		config: newProviderConfig(),
	}
}

// Provider implements the internal server.Provider interface.
type Provider struct {
	mu     sync.Mutex
	config *config
}

type config struct {
	mu        sync.Mutex
	Transport *embeddedTransportV1
}

func newProviderConfig() *config {
	return &config{
		mu:        sync.Mutex{},
		Transport: newEmbeddedTransport(),
	}
}

// Schema is the provider user configuration schema
func (p *Provider) Schema() *tfprotov5.Schema {
	return &tfprotov5.Schema{
		Version: 1,
		Block: &tfprotov5.SchemaBlock{
			Version: 1,
			Attributes: []*tfprotov5.SchemaAttribute{
				p.config.Transport.SchemaAttributeTransport(),
			},
		},
	}
}

// MetaSchema is the schema for the providers metadata
func (p *Provider) MetaSchema() *tfprotov5.Schema {
	return nil
}

// PrepareConfig is called to give a provider a chance to modify the configuration
// the user specified before validation.
func (p *Provider) PrepareConfig(ctx context.Context, req *tfprotov5.PrepareProviderConfigRequest) (*tfprotov5.PrepareProviderConfigResponse, error) {
	res := &tfprotov5.PrepareProviderConfigResponse{
		Diagnostics:    []*tfprotov5.Diagnostic{},
		PreparedConfig: req.Config,
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	return res, nil
}

// Configure is called to pass the user-specified provider configuration to the
// provider.
func (p *Provider) Configure(ctx context.Context, req *tfprotov5.ConfigureProviderRequest) (*tfprotov5.ConfigureProviderResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	res := &tfprotov5.ConfigureProviderResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	err := unmarshal(p.config, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	// Pull in any environment config
	p.config.Transport.FromEnvironment()

	return res, nil
}

// Stop is called when Terraform would like providers to shut down as quickly
// as possible, and usually represents an interrupt.
func (p *Provider) Stop(ctx context.Context, req *tfprotov5.StopProviderRequest) (*tfprotov5.StopProviderResponse, error) {
	return &tfprotov5.StopProviderResponse{}, nil
}

// Config returns the providers configuration as a Terraform5Value
func (p *Provider) Config() tftypes.Value {
	return p.config.Terraform5Value()
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (c *config) FromTerraform5Value(val tftypes.Value) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	vals := map[string]tftypes.Value{}
	err := val.As(&vals)
	if err != nil {
		return wrapErrWithDiagnostics(err, "invalid configuration", "unable to unmarshal provider configuration")
	}

	if !vals["transport"].IsKnown() {
		return nil
	}

	return c.Transport.FromTerraform5Value(vals["transport"])
}

// Terraform5Type is the provider as a tftypes.Type
func (c *config) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"transport": c.Transport.Terraform5Type(),
	}}
}

// Terraform5Value is the provider as a tftypes.Value
func (c *config) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(c.Terraform5Type(), map[string]tftypes.Value{
		"transport": c.Transport.Terraform5Value(),
	})
}

// Copy returns a copy of the provider configuration.  We always return a copy
// so that parallel resources don't race for the pointer
func (c *config) Copy() (*config, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error

	newCopy := newProviderConfig()
	newCopy.Transport, err = c.Transport.Copy()

	return newCopy, err
}
