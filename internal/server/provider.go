package server

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tftypes"
)

// Provider is our Provider meta server
type Provider interface {
	Schema() *tfprotov5.Schema
	MetaSchema() *tfprotov5.Schema
	PrepareConfig(context.Context, *tfprotov5.PrepareProviderConfigRequest) (*tfprotov5.PrepareProviderConfigResponse, error)
	Configure(context.Context, *tfprotov5.ConfigureProviderRequest) (*tfprotov5.ConfigureProviderResponse, error)
	Stop(context.Context, *tfprotov5.StopProviderRequest) (*tfprotov5.StopProviderResponse, error)
	Config() tftypes.Value
}
