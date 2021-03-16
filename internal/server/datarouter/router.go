package datarouter

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tftypes"
)

type errUnsupportedDataSource string

func (e errUnsupportedDataSource) Error() string {
	return "unsupported data source: " + string(e)
}

type errSetProviderConfig struct {
	err error
}

func (e *errSetProviderConfig) Unwrap() error {
	return e.err
}

func (e *errSetProviderConfig) Error() string {
	return fmt.Sprintf("setting provider config on data source: %s", e.err.Error())
}

func newErrSetProviderConfig(err error) error {
	return &errSetProviderConfig{err: err}
}

// DataSource is the DataSource
type DataSource interface {
	tfprotov5.DataSourceServer
	Name() string
	Schema() *tfprotov5.Schema
	SetProviderConfig(tftypes.Value) error
}

// RouterOpt is a functional option for the data router
type RouterOpt func(Router) Router

// Router routes requests to the various data resources
type Router struct {
	dataSources map[string]DataSource
}

// ValidateDataSourceConfig validates the data sources config
func (r Router) ValidateDataSourceConfig(ctx context.Context, req *tfprotov5.ValidateDataSourceConfigRequest, meta tftypes.Value) (*tfprotov5.ValidateDataSourceConfigResponse, error) {
	ds, ok := r.dataSources[req.TypeName]
	if !ok {
		return nil, errUnsupportedDataSource(req.TypeName)
	}

	err := ds.SetProviderConfig(meta)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	return ds.ValidateDataSourceConfig(ctx, req)
}

// ReadDataSource refreshes the data sources state
func (r Router) ReadDataSource(ctx context.Context, req *tfprotov5.ReadDataSourceRequest, meta tftypes.Value) (*tfprotov5.ReadDataSourceResponse, error) {
	ds, ok := r.dataSources[req.TypeName]
	if !ok {
		return nil, errUnsupportedDataSource(req.TypeName)
	}

	err := ds.SetProviderConfig(meta)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	return ds.ReadDataSource(ctx, req)
}

// New takes zero or more functional options and return a new DataSource router
func New(opts ...RouterOpt) Router {
	r := new()
	for _, opt := range opts {
		r = opt(r)
	}

	return r
}

func new() Router {
	return Router{
		dataSources: map[string]DataSource{},
	}
}

// RegisterDataSource registers a DataSource with the Router
func RegisterDataSource(data DataSource) func(Router) Router {
	return func(router Router) Router {
		router.dataSources[data.Name()] = data

		return router
	}
}

// Schemas returns the data router schemas
func (r Router) Schemas() map[string]*tfprotov5.Schema {
	schemas := map[string]*tfprotov5.Schema{}
	for name, dataSource := range r.dataSources {
		schemas[name] = dataSource.Schema()
	}

	return schemas
}
