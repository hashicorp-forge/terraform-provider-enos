package server

import (
	"context"

	"github.com/hashicorp/enos-provider/internal/server/datarouter"
	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

var _ tfprotov5.ProviderServer = (*Server)(nil)

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

func (s Server) GetProviderSchema(ctx context.Context, req *tfprotov5.GetProviderSchemaRequest) (*tfprotov5.GetProviderSchemaResponse, error) {
	return &tfprotov5.GetProviderSchemaResponse{
		Provider:          s.provider.Schema(),
		ProviderMeta:      s.provider.MetaSchema(),
		ResourceSchemas:   s.resourceRouter.Schemas(),
		DataSourceSchemas: s.dataRouter.Schemas(),
	}, nil
}

func (s Server) PrepareProviderConfig(ctx context.Context, req *tfprotov5.PrepareProviderConfigRequest) (*tfprotov5.PrepareProviderConfigResponse, error) {
	return s.provider.PrepareConfig(ctx, req)
}

func (s Server) ConfigureProvider(ctx context.Context, req *tfprotov5.ConfigureProviderRequest) (*tfprotov5.ConfigureProviderResponse, error) {
	return s.provider.Configure(ctx, req)
}

func (s Server) StopProvider(ctx context.Context, req *tfprotov5.StopProviderRequest) (*tfprotov5.StopProviderResponse, error) {
	return s.provider.Stop(ctx, req)
}

func (s Server) ValidateResourceTypeConfig(ctx context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	return s.resourceRouter.ValidateResourceTypeConfig(ctx, req)
}

func (s Server) UpgradeResourceState(ctx context.Context, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
	return s.resourceRouter.UpgradeResourceState(ctx, req)
}

func (s Server) ReadResource(ctx context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	return s.resourceRouter.ReadResource(ctx, req)
}

func (s Server) PlanResourceChange(ctx context.Context, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, error) {
	return s.resourceRouter.PlanResourceChange(ctx, req)
}

func (s Server) ApplyResourceChange(ctx context.Context, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
	return s.resourceRouter.ApplyResourceChange(ctx, req)
}

func (s Server) ImportResourceState(ctx context.Context, req *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
	return s.resourceRouter.ImportResourceState(ctx, req)
}

func (s Server) ValidateDataSourceConfig(ctx context.Context, req *tfprotov5.ValidateDataSourceConfigRequest) (*tfprotov5.ValidateDataSourceConfigResponse, error) {
	return s.dataRouter.ValidateDataSourceConfig(ctx, req)
}

func (s Server) ReadDataSource(ctx context.Context, req *tfprotov5.ReadDataSourceRequest) (*tfprotov5.ReadDataSourceResponse, error) {
	return s.dataRouter.ReadDataSource(ctx, req)
}
