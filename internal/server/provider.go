package server

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Provider is our Provider meta server
type Provider interface {
	Schema() *tfprotov6.Schema
	MetaSchema() *tfprotov6.Schema
	Validate(context.Context, *tfprotov6.ValidateProviderConfigRequest) (*tfprotov6.ValidateProviderConfigResponse, error)
	Configure(context.Context, *tfprotov6.ConfigureProviderRequest) (*tfprotov6.ConfigureProviderResponse, error)
	Stop(context.Context, *tfprotov6.StopProviderRequest) (*tfprotov6.StopProviderResponse, error)
	Config() tftypes.Value
}
