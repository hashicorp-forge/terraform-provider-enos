// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/diags"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/server"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/state"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	enosDebugDataRootDirEnvVarKey = "ENOS_DEBUG_DATA_ROOT_DIR" // env var for setting the debug_data_root_dir.
)

var (
	_ server.Provider    = (*Provider)(nil)
	_ state.Serializable = (*config)(nil)
)

// newProvider returns a new instance of the plugin provider server.
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
	mu               sync.Mutex
	Transport        *embeddedTransportV1
	DebugDataRootDir *tfString
}

func newProviderConfig() *config {
	return &config{
		mu:               sync.Mutex{},
		Transport:        newEmbeddedTransport(),
		DebugDataRootDir: newTfString(),
	}
}

// Schema is the provider user configuration schema.
func (p *Provider) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			Version: 1,
			Attributes: []*tfprotov6.SchemaAttribute{
				p.config.Transport.SchemaAttributeTransport(supportsSSH | supportsK8s | supportsNomad),
				{
					Name:     "debug_data_root_dir",
					Type:     tftypes.String,
					Optional: true,
					Description: `The root directory where failure diagnostics files (e.g. application log files) are saved.
If configured and the directory does not exist, it will be created.
If the directory is not configured, diagnostic files will not be saved locally.`,
				},
			},
			DescriptionKind: providerDescriptionKind,
			Description:     providerDescription,
		},
	}
}

// MetaSchema is the schema for the providers metadata.
func (p *Provider) MetaSchema() *tfprotov6.Schema {
	return nil
}

// Validate is called to give a provider a chance to validate the configuration.
func (p *Provider) Validate(ctx context.Context, req *tfprotov6.ValidateProviderConfigRequest) (*tfprotov6.ValidateProviderConfigResponse, error) {
	res := &tfprotov6.ValidateProviderConfigResponse{
		Diagnostics:    []*tfprotov6.Diagnostic{},
		PreparedConfig: req.Config,
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return res, ctx.Err()
	default:
	}

	cfg := newProviderConfig()
	err := unmarshal(cfg, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return res, err
	}

	if dir, ok := cfg.DebugDataRootDir.Get(); ok {
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return res, nil
			}
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Validation Error", err))

			return res, nil
		}
		if !info.IsDir() {
			res.Diagnostics = append(res.Diagnostics,
				diags.ErrToDiagnostic("Validation Error",
					ValidationError("configured diagnostics dir is not a directory", "debug_data_root_dir"),
				))

			return res, nil
		}
	}

	return res, nil
}

// Configure is called to pass the user-specified provider configuration to the
// provider.
func (p *Provider) Configure(ctx context.Context, req *tfprotov6.ConfigureProviderRequest) (*tfprotov6.ConfigureProviderResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	res := &tfprotov6.ConfigureProviderResponse{
		Diagnostics: []*tfprotov6.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return res, ctx.Err()
	default:
	}

	err := unmarshal(p.config, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return res, err
	}

	// the env var ENOS_DEBUG_DATA_ROOT_DIR should override the value configured in the provider block
	if debugDir, ok := os.LookupEnv(enosDebugDataRootDirEnvVarKey); ok {
		p.config.DebugDataRootDir.Set(debugDir)
	}

	if dir, ok := p.config.DebugDataRootDir.Get(); ok {
		if _, err := os.Stat(dir); err != nil {
			if os.IsNotExist(err) {
				err := os.MkdirAll(dir, 0o755)
				if err == nil {
					return res, nil
				}
			}
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Provider Config Error", err))

			return res, nil
		}
	}

	return res, nil
}

// Stop is called when Terraform would like providers to shut down as quickly
// as possible, and usually represents an interrupt.
func (p *Provider) Stop(ctx context.Context, req *tfprotov6.StopProviderRequest) (*tfprotov6.StopProviderResponse, error) {
	return &tfprotov6.StopProviderResponse{}, nil
}

// Config returns the providers configuration as a Terraform5Value.
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
		return fmt.Errorf("failed to unmarshal provider configuration, due to: %w", err)
	}

	if !vals["transport"].IsKnown() {
		return nil
	}

	err = c.Transport.FromTerraform5Value(vals["transport"])
	if err != nil {
		return fmt.Errorf("failed to unmarshal transport configuration, due to: %w", err)
	}

	if !vals["debug_data_root_dir"].IsKnown() {
		return nil
	}

	err = c.DebugDataRootDir.FromTFValue(vals["debug_data_root_dir"])
	if err != nil {
		return fmt.Errorf("failed to unmarshal [debug_data_root_dir], due to: %w", err)
	}

	return err
}

// Terraform5Type is the provider as a tftypes.Type.
func (c *config) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"transport":           c.Transport.Terraform5Type(),
		"debug_data_root_dir": c.DebugDataRootDir.TFType(),
	}}
}

// Terraform5Value is the provider as a tftypes.Value.
func (c *config) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(c.Terraform5Type(), map[string]tftypes.Value{
		"transport":           c.Transport.Terraform5Value(),
		"debug_data_root_dir": c.DebugDataRootDir.TFValue(),
	})
}

// Copy returns a copy of the provider configuration.  We always return a copy
// so that parallel resources don't race for the pointer.
func (c *config) Copy() (*config, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error

	newCopy := newProviderConfig()
	newCopy.Transport, err = c.Transport.Copy()
	dir := newTfString()
	dir.Set(c.DebugDataRootDir.Val)
	newCopy.DebugDataRootDir = dir

	return newCopy, err
}
