package plugin

import (
	"context"

	"github.com/hashicorp/enos-provider/internal/server"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

var _ server.Provider = (*Provider)(nil)

func NewProvider() Provider {
	return Provider{}
}

type Provider struct {
}

func (p Provider) Schema() *tfprotov5.Schema {
	return &tfprotov5.Schema{
		Version: 1,
		Block: &tfprotov5.SchemaBlock{
			Version: 1,
		},
	}
}

func (p Provider) MetaSchema() *tfprotov5.Schema {
	return nil
}

func (p Provider) PrepareConfig(ctx context.Context, req *tfprotov5.PrepareProviderConfigRequest) (*tfprotov5.PrepareProviderConfigResponse, error) {
	return &tfprotov5.PrepareProviderConfigResponse{
		PreparedConfig: req.Config,
	}, nil
}

func (p Provider) Configure(ctx context.Context, req *tfprotov5.ConfigureProviderRequest) (*tfprotov5.ConfigureProviderResponse, error) {
	return &tfprotov5.ConfigureProviderResponse{}, nil
}

func (p Provider) Stop(ctx context.Context, req *tfprotov5.StopProviderRequest) (*tfprotov5.StopProviderResponse, error) {
	return &tfprotov5.StopProviderResponse{}, nil
}
