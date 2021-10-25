package resourcerouter

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type errUnsupportedResource string

func (e errUnsupportedResource) Error() string {
	return "unsupported resource: " + string(e)
}

type errSetProviderConfig struct {
	err error
}

func (e *errSetProviderConfig) Unwrap() error {
	return e.err
}

func (e *errSetProviderConfig) Error() string {
	return fmt.Sprintf("setting provider config on resource: %s", e.err.Error())
}

func newErrSetProviderConfig(err error) error {
	return &errSetProviderConfig{err: err}
}

// Resource represents a Terraform resource
type Resource interface {
	tfprotov6.ResourceServer
	Name() string
	Schema() *tfprotov6.Schema
	SetProviderConfig(tftypes.Value) error
}

// RouterOpt is a functional option for the router constructor
type RouterOpt func(Router) Router

// New takes zero or more functional options and returns a new Router
func New(opts ...RouterOpt) Router {
	r := new()
	for _, opt := range opts {
		r = opt(r)
	}

	return r
}

// RegisterResource is a functional option that register a new Resource with
// the Router
func RegisterResource(resource Resource) func(Router) Router {
	return func(router Router) Router {
		router.resources[resource.Name()] = resource

		return router
	}
}

func new() Router {
	return Router{
		resources: map[string]Resource{},
	}
}

// Router routes the requests the resource servers
type Router struct {
	resources map[string]Resource
}

// ValidateResourceConfig validates the resource's config
func (r Router) ValidateResourceConfig(ctx context.Context, req *tfprotov6.ValidateResourceConfigRequest, providerConfig tftypes.Value) (*tfprotov6.ValidateResourceConfigResponse, error) {
	res, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := res.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	return res.ValidateResourceConfig(ctx, req)
}

// UpgradeResourceState upgrades the state when migrating from an old version to a new version
func (r Router) UpgradeResourceState(ctx context.Context, req *tfprotov6.UpgradeResourceStateRequest, providerConfig tftypes.Value) (*tfprotov6.UpgradeResourceStateResponse, error) {
	res, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := res.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	return res.UpgradeResourceState(ctx, req)
}

// ReadResource refreshes the resource's state
func (r Router) ReadResource(ctx context.Context, req *tfprotov6.ReadResourceRequest, providerConfig tftypes.Value) (*tfprotov6.ReadResourceResponse, error) {
	res, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := res.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	return res.ReadResource(ctx, req)
}

// PlanResourceChange proposes a new resource state
func (r Router) PlanResourceChange(ctx context.Context, req *tfprotov6.PlanResourceChangeRequest, providerConfig tftypes.Value) (*tfprotov6.PlanResourceChangeResponse, error) {
	res, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := res.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	return res.PlanResourceChange(ctx, req)
}

// ApplyResourceChange applies the newly planned resource state
func (r Router) ApplyResourceChange(ctx context.Context, req *tfprotov6.ApplyResourceChangeRequest, providerConfig tftypes.Value) (*tfprotov6.ApplyResourceChangeResponse, error) {
	res, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := res.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	return res.ApplyResourceChange(ctx, req)
}

// ImportResourceState fetches the resource from an ID and adds it to the state
func (r Router) ImportResourceState(ctx context.Context, req *tfprotov6.ImportResourceStateRequest, providerConfig tftypes.Value) (*tfprotov6.ImportResourceStateResponse, error) {
	res, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := res.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	return res.ImportResourceState(ctx, req)
}

// Schemas returns the data router schemas
func (r Router) Schemas() map[string]*tfprotov6.Schema {
	schemas := map[string]*tfprotov6.Schema{}
	for name, resource := range r.resources {
		schemas[name] = resource.Schema()
	}

	return schemas
}
