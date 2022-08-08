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

// WithDefaultResourceRouter creates a server opt that registers all the default resources and
// optionally any provided overrides (or additional, non-default resources). The optional overrides
// argument is useful if you need to override a resource in a test.
func WithDefaultResourceRouter(overrides ...rr.Resource) func(server.Server) server.Server {
	return server.RegisterResourceRouter(buildResourceRouter(overrides...))
}

// WithDefaultDataRouter creates a server opt that registers all the default datasources and
// optionally any provided overrides (or additional, non-default resources). The optional overrides
// argument is useful if you need to override a datasource in a test.
func WithDefaultDataRouter(overrides ...dr.DataSource) func(server.Server) server.Server {
	return server.RegisterDataRouter(buildDataRouter(overrides...))
}

// helpers

// defaultDataSources returns a slice of all the data sources that the provider supports
func defaultDataSources() []dr.DataSource {
	return []dr.DataSource{
		newEnvironment(),
		newArtifactoryItem(),
		newKubernetesPods(),
	}
}

// defaultResources returns a slice of all the resources that the provider supports
func defaultResources() []rr.Resource {
	return []rr.Resource{
		newFile(),
		newLocalKindLoadImage(),
		newLocalKindCluster(),
		newLocalExec(),
		newRemoteExec(),
		newBundleInstall(),
		newVaultStart(),
		newVaultInit(),
		newVaultUnseal(),
		newConsulStart(),
		newBoundaryStart(),
		newBoundaryInit(),
	}
}

func buildResourceRouter(resourceOverrides ...rr.Resource) rr.Router {
	var opts []rr.RouterOpt
	for _, resource := range defaultResources() {
		opts = append(opts, rr.RegisterResource(resource))
	}
	for _, resource := range resourceOverrides {
		opts = append(opts, rr.RegisterResource(resource))
	}
	return rr.New(opts...)
}

func buildDataRouter(dataSourceOverrides ...dr.DataSource) dr.Router {
	var opts []dr.RouterOpt
	for _, datasource := range defaultDataSources() {
		opts = append(opts, dr.RegisterDataSource(datasource))
	}
	for _, datasource := range dataSourceOverrides {
		opts = append(opts, dr.RegisterDataSource(datasource))
	}

	return dr.New(opts...)
}
