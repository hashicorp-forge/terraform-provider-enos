package datarouter

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

// DataSourceServerAdapter Adapter for a tfprotov6.DataSourceServer removing the error return type from all methods.
type DataSourceServerAdapter interface {
	ValidateDataResourceConfig(context.Context, tfprotov6.ValidateDataResourceConfigRequest, *tfprotov6.ValidateDataResourceConfigResponse)
	ReadDataSource(context.Context, tfprotov6.ReadDataSourceRequest, *tfprotov6.ReadDataSourceResponse)
}

// DataSource is the DataSource.
type DataSource interface {
	DataSourceServerAdapter
	Name() string
	Schema() *tfprotov6.Schema
	SetProviderConfig(tftypes.Value) error
}

// RouterOpt is a functional option for the data router.
type RouterOpt func(Router) Router

// Router routes requests to the various data resources.
type Router struct {
	dataSources map[string]DataSource
}

// ValidateDataResourceConfig validates the data sources config.
func (r Router) ValidateDataResourceConfig(ctx context.Context, req *tfprotov6.ValidateDataResourceConfigRequest, meta tftypes.Value) (*tfprotov6.ValidateDataResourceConfigResponse, error) {
	res := &tfprotov6.ValidateDataResourceConfigResponse{
		Diagnostics: []*tfprotov6.Diagnostic{},
	}
	ds, ok := r.dataSources[req.TypeName]
	if !ok {
		return nil, errUnsupportedDataSource(req.TypeName)
	}

	err := ds.SetProviderConfig(meta)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	ds.ValidateDataResourceConfig(ctx, *req, res)

	return res, nil
}

// ReadDataSource refreshes the data sources state.
func (r Router) ReadDataSource(ctx context.Context, req *tfprotov6.ReadDataSourceRequest, meta tftypes.Value) (*tfprotov6.ReadDataSourceResponse, error) {
	res := &tfprotov6.ReadDataSourceResponse{
		Diagnostics: []*tfprotov6.Diagnostic{},
	}

	ds, ok := r.dataSources[req.TypeName]
	if !ok {
		return nil, errUnsupportedDataSource(req.TypeName)
	}

	err := ds.SetProviderConfig(meta)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	ds.ReadDataSource(ctx, *req, res)

	return res, nil
}

// New takes zero or more functional options and return a new DataSource router.
func New(opts ...RouterOpt) Router {
	r := newRouter()
	for _, opt := range opts {
		r = opt(r)
	}

	return r
}

func newRouter() Router {
	return Router{
		dataSources: map[string]DataSource{},
	}
}

// RegisterDataSource registers a DataSource with the Router.
func RegisterDataSource(data DataSource) func(Router) Router {
	return func(router Router) Router {
		router.dataSources[data.Name()] = data

		return router
	}
}

// Schemas returns the data router schemas.
func (r Router) Schemas() map[string]*tfprotov6.Schema {
	schemas := map[string]*tfprotov6.Schema{}
	for name, dataSource := range r.dataSources {
		schemas[name] = dataSource.Schema()
	}

	return schemas
}
