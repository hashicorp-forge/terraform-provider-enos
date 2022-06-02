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

// ResourceServerAdapter Adapter for a tfprotov6.ResourceServer removing the error return type from all methods.
type ResourceServerAdapter interface {
	ValidateResourceConfig(context.Context, tfprotov6.ValidateResourceConfigRequest, *tfprotov6.ValidateResourceConfigResponse)
	UpgradeResourceState(context.Context, tfprotov6.UpgradeResourceStateRequest, *tfprotov6.UpgradeResourceStateResponse)
	ReadResource(context.Context, tfprotov6.ReadResourceRequest, *tfprotov6.ReadResourceResponse)
	PlanResourceChange(context.Context, tfprotov6.PlanResourceChangeRequest, *tfprotov6.PlanResourceChangeResponse)
	ApplyResourceChange(context.Context, tfprotov6.ApplyResourceChangeRequest, *tfprotov6.ApplyResourceChangeResponse)
	ImportResourceState(context.Context, tfprotov6.ImportResourceStateRequest, *tfprotov6.ImportResourceStateResponse)
}

// Resource represents a Terraform resource
type Resource interface {
	ResourceServerAdapter
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
	res := &tfprotov6.ValidateResourceConfigResponse{
		Diagnostics: []*tfprotov6.Diagnostic{},
	}
	resource, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := resource.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}
	resource.ValidateResourceConfig(ctx, *req, res)
	return res, nil
}

// UpgradeResourceState upgrades the state when migrating from an old version to a new version
func (r Router) UpgradeResourceState(ctx context.Context, req *tfprotov6.UpgradeResourceStateRequest, providerConfig tftypes.Value) (*tfprotov6.UpgradeResourceStateResponse, error) {
	res := &tfprotov6.UpgradeResourceStateResponse{
		Diagnostics: []*tfprotov6.Diagnostic{},
	}

	resource, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := resource.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	resource.UpgradeResourceState(ctx, *req, res)
	return res, nil
}

// ReadResource refreshes the resource's state
func (r Router) ReadResource(ctx context.Context, req *tfprotov6.ReadResourceRequest, providerConfig tftypes.Value) (*tfprotov6.ReadResourceResponse, error) {
	res := &tfprotov6.ReadResourceResponse{
		Diagnostics: []*tfprotov6.Diagnostic{},
	}

	resource, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := resource.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	resource.ReadResource(ctx, *req, res)
	return res, nil
}

// PlanResourceChange proposes a new resource state
func (r Router) PlanResourceChange(ctx context.Context, req *tfprotov6.PlanResourceChangeRequest, providerConfig tftypes.Value) (*tfprotov6.PlanResourceChangeResponse, error) {
	res := &tfprotov6.PlanResourceChangeResponse{
		Diagnostics: []*tfprotov6.Diagnostic{},
	}

	resource, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := resource.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	resource.PlanResourceChange(ctx, *req, res)
	return res, nil
}

// ApplyResourceChange applies the newly planned resource state
func (r Router) ApplyResourceChange(ctx context.Context, req *tfprotov6.ApplyResourceChangeRequest, providerConfig tftypes.Value) (*tfprotov6.ApplyResourceChangeResponse, error) {
	res := &tfprotov6.ApplyResourceChangeResponse{
		Diagnostics: []*tfprotov6.Diagnostic{},
	}

	resource, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := resource.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	resource.ApplyResourceChange(ctx, *req, res)
	return res, nil
}

// ImportResourceState fetches the resource from an ID and adds it to the state
func (r Router) ImportResourceState(ctx context.Context, req *tfprotov6.ImportResourceStateRequest, providerConfig tftypes.Value) (*tfprotov6.ImportResourceStateResponse, error) {
	res := &tfprotov6.ImportResourceStateResponse{
		ImportedResources: []*tfprotov6.ImportedResource{},
		Diagnostics:       []*tfprotov6.Diagnostic{},
	}

	resource, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := resource.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	resource.ImportResourceState(ctx, *req, res)
	return res, nil
}

// Schemas returns the data router schemas
func (r Router) Schemas() map[string]*tfprotov6.Schema {
	schemas := map[string]*tfprotov6.Schema{}
	for name, resource := range r.resources {
		schemas[name] = resource.Schema()
	}

	return schemas
}
