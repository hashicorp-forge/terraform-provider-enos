package plugin

import (
	"github.com/hashicorp/enos-provider/internal/server"
	dr "github.com/hashicorp/enos-provider/internal/server/datarouter"
	rr "github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// Server returns a default instance of our ProviderServer
func Server() tfprotov6.ProviderServer {
	return server.New(
		server.RegisterProvider(newProvider()),
		WithDefaultDataRouter(),
		WithDefaultResourceRouter(),
	)
}

func WithDefaultResourceRouter() func(server.Server) server.Server {
	return server.RegisterResourceRouter(rr.New(
		rr.RegisterResource(newFile()),
		rr.RegisterResource(newLocalKindLoadImage()),
		rr.RegisterResource(newLocalKindCluster()),
		rr.RegisterResource(newLocalExec()),
		rr.RegisterResource(newRemoteExec()),
		rr.RegisterResource(newBundleInstall()),
		rr.RegisterResource(newVaultStart()),
		rr.RegisterResource(newVaultInit()),
		rr.RegisterResource(newVaultUnseal()),
		rr.RegisterResource(newConsulStart()),
		rr.RegisterResource(newBoundaryStart()),
		rr.RegisterResource(newBoundaryInit()),
	))
}

func WithDefaultDataRouter() func(server.Server) server.Server {
	return server.RegisterDataRouter(dr.New(
		dr.RegisterDataSource(newEnvironment()),
		dr.RegisterDataSource(newArtifactoryItem()),
		dr.RegisterDataSource(newKubernetesPods()),
	))
}
