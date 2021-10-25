package server

import (
	"context"

	"github.com/hashicorp/enos-provider/internal/server/datarouter"
	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// Server is our gRPC ProviderServer
type Server struct {
	provider       Provider
	resourceRouter resourcerouter.Router
	dataRouter     datarouter.Router
}

// Opt is a functional option for the provider server
type Opt func(Server) Server

// New takes zero or more functional options and return a new Server
func New(opts ...Opt) Server {
	s := Server{}
	for _, opt := range opts {
		s = opt(s)
	}

	return s
}

// RegisterProvider is a functional option that registers the Provider meta server
func RegisterProvider(provider Provider) func(Server) Server {
	return func(server Server) Server {
		server.provider = provider

		return server
	}
}

// RegisterDataRouter is a functional option that registers the data source router
func RegisterDataRouter(router datarouter.Router) func(Server) Server {
	return func(server Server) Server {
		server.dataRouter = router

		return server
	}
}

// RegisterResourceRouter is a functional option that registers the resource router
func RegisterResourceRouter(router resourcerouter.Router) func(Server) Server {
	return func(server Server) Server {
		server.resourceRouter = router

		return server
	}
}

func (s Server) GetProviderSchema(ctx context.Context, req *tfprotov6.GetProviderSchemaRequest) (*tfprotov6.GetProviderSchemaResponse, error) {
	return &tfprotov6.GetProviderSchemaResponse{
		Provider:          s.provider.Schema(),
		ProviderMeta:      s.provider.MetaSchema(),
		ResourceSchemas:   s.resourceRouter.Schemas(),
		DataSourceSchemas: s.dataRouter.Schemas(),
	}, nil
}

func (s Server) ValidateProviderConfig(ctx context.Context, req *tfprotov6.ValidateProviderConfigRequest) (*tfprotov6.ValidateProviderConfigResponse, error) {
	return s.provider.Validate(ctx, req)
}

func (s Server) ConfigureProvider(ctx context.Context, req *tfprotov6.ConfigureProviderRequest) (*tfprotov6.ConfigureProviderResponse, error) {
	return s.provider.Configure(ctx, req)
}

func (s Server) StopProvider(ctx context.Context, req *tfprotov6.StopProviderRequest) (*tfprotov6.StopProviderResponse, error) {
	return s.provider.Stop(ctx, req)
}

func (s Server) ValidateResourceConfig(ctx context.Context, req *tfprotov6.ValidateResourceConfigRequest) (*tfprotov6.ValidateResourceConfigResponse, error) {
	return s.resourceRouter.ValidateResourceConfig(ctx, req, s.provider.Config())
}

func (s Server) UpgradeResourceState(ctx context.Context, req *tfprotov6.UpgradeResourceStateRequest) (*tfprotov6.UpgradeResourceStateResponse, error) {
	return s.resourceRouter.UpgradeResourceState(ctx, req, s.provider.Config())
}

func (s Server) ReadResource(ctx context.Context, req *tfprotov6.ReadResourceRequest) (*tfprotov6.ReadResourceResponse, error) {
	return s.resourceRouter.ReadResource(ctx, req, s.provider.Config())
}

func (s Server) PlanResourceChange(ctx context.Context, req *tfprotov6.PlanResourceChangeRequest) (*tfprotov6.PlanResourceChangeResponse, error) {
	return s.resourceRouter.PlanResourceChange(ctx, req, s.provider.Config())
}

func (s Server) ApplyResourceChange(ctx context.Context, req *tfprotov6.ApplyResourceChangeRequest) (*tfprotov6.ApplyResourceChangeResponse, error) {
	return s.resourceRouter.ApplyResourceChange(ctx, req, s.provider.Config())
}

func (s Server) ImportResourceState(ctx context.Context, req *tfprotov6.ImportResourceStateRequest) (*tfprotov6.ImportResourceStateResponse, error) {
	return s.resourceRouter.ImportResourceState(ctx, req, s.provider.Config())
}

func (s Server) ValidateDataResourceConfig(ctx context.Context, req *tfprotov6.ValidateDataResourceConfigRequest) (*tfprotov6.ValidateDataResourceConfigResponse, error) {
	return s.dataRouter.ValidateDataResourceConfig(ctx, req, s.provider.Config())
}

func (s Server) ReadDataSource(ctx context.Context, req *tfprotov6.ReadDataSourceRequest) (*tfprotov6.ReadDataSourceResponse, error) {
	return s.dataRouter.ReadDataSource(ctx, req, s.provider.Config())
}
