// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/server"
	dr "github.com/hashicorp-forge/terraform-provider-enos/internal/server/datarouter"
	rr "github.com/hashicorp-forge/terraform-provider-enos/internal/server/resourcerouter"
)

// Server returns a default instance of our ProviderServer.
func Server() tfprotov6.ProviderServer {
	p := newProvider()

	return server.New(
		server.RegisterProvider(p),
		WithDefaultDataRouter(),
		WithDefaultResourceRouter(p.Registry()),
	)
}

// WithDefaultResourceRouter creates a server opt that registers all the default resources and
// optionally any provided overrides (or additional, non-default resources). The optional overrides
// argument is useful if you need to override a resource in a test.
//
// registry is the provider-level transport target registry; it may be nil (e.g. in tests that do
// not need log collection from non-failing resources).
func WithDefaultResourceRouter(registry any, overrides ...rr.Resource) func(server.Server) server.Server {
	return server.RegisterResourceRouter(buildResourceRouter(registry, overrides...))
}

// WithDefaultDataRouter creates a server opt that registers all the default datasources and
// optionally any provided overrides (or additional, non-default resources). The optional overrides
// argument is useful if you need to override a datasource in a test.
func WithDefaultDataRouter(overrides ...dr.DataSource) func(server.Server) server.Server {
	return server.RegisterDataRouter(buildDataRouter(overrides...))
}

// helpers

// defaultDataSources returns a slice of all the data sources that the provider supports.
func defaultDataSources() []dr.DataSource {
	return []dr.DataSource{
		newArtifactoryItem(),
		newEnvironment(),
		newKubernetesPods(),
	}
}

// defaultResources returns a slice of all the resources that the provider supports.
func defaultResources() []rr.Resource {
	return []rr.Resource{
		newBoundaryInit(),
		newBoundaryStart(),
		newBundleInstall(),
		newConsulStart(),
		newFile(),
		newHostInfo(),
		newLocalKindCluster(),
		newLocalKindLoadImage(),
		newLocalExec(),
		newRemoteExec(),
		newUser(),
		newVaultInit(),
		newVaultStart(),
		newVaultUnseal(),
	}
}

func buildResourceRouter(registry any, resourceOverrides ...rr.Resource) rr.Router {
	defaultResources := defaultResources()
	// +1 for WithRegistry opt
	opts := make([]rr.RouterOpt, 0, len(defaultResources)+len(resourceOverrides)+1)

	for i := range defaultResources {
		opts = append(opts, rr.RegisterResource(defaultResources[i]))
	}
	for i := range resourceOverrides {
		opts = append(opts, rr.RegisterResource(resourceOverrides[i]))
	}
	if registry != nil {
		opts = append(opts, rr.WithRegistry(registry))
	}

	return rr.New(opts...)
}

func buildDataRouter(dataSourceOverrides ...dr.DataSource) dr.Router {
	defaultDataSources := defaultDataSources()
	opts := make([]dr.RouterOpt, len(defaultDataSources)+len(dataSourceOverrides))
	count := 0

	for i := range defaultDataSources {
		opts[count] = dr.RegisterDataSource(defaultDataSources[i])
		count++
	}
	for i := range dataSourceOverrides {
		opts[count] = dr.RegisterDataSource(dataSourceOverrides[i])
		count++
	}

	return dr.New(opts...)
}
