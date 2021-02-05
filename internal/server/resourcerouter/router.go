package resourcerouter

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

type errUnsupportedResource string

func (e errUnsupportedResource) Error() string {
	return "unsupported resource: " + string(e)
}

type Resource interface {
	tfprotov5.ResourceServer
	Name() string
	Schema() *tfprotov5.Schema
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

var _ tfprotov5.ResourceServer = (*Router)(nil)

// ValidateResourceTypeConfig validates the resource's config
func (r Router) ValidateResourceTypeConfig(ctx context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	res, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}
	return res.ValidateResourceTypeConfig(ctx, req)
}

// UpgradeResourceState upgrades the state when migrating from an old version to a new version
func (r Router) UpgradeResourceState(ctx context.Context, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
	res, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}
	return res.UpgradeResourceState(ctx, req)
}

// ReadResource refreshes the resource's state
func (r Router) ReadResource(ctx context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	res, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}
	return res.ReadResource(ctx, req)
}

// PlanResourceChange proposes a new resource state
func (r Router) PlanResourceChange(ctx context.Context, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, error) {
	res, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}
	return res.PlanResourceChange(ctx, req)
}

// ApplyResourceChange applies the newly planned resource state
func (r Router) ApplyResourceChange(ctx context.Context, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
	res, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}
	return res.ApplyResourceChange(ctx, req)
}

// ImportResourceState fetches the resource from an ID and adds it to the state
func (r Router) ImportResourceState(ctx context.Context, req *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
	res, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}
	return res.ImportResourceState(ctx, req)
}

// Schemas returns the data router schemas
func (r Router) Schemas() map[string]*tfprotov5.Schema {
	schemas := map[string]*tfprotov5.Schema{}
	for name, resource := range r.resources {
		schemas[name] = resource.Schema()
	}

	return schemas
}
