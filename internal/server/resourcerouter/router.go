// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resourcerouter

import (
	"context"

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
	return "setting provider config on resource: " + e.err.Error()
}

func newErrSetProviderConfig(err error) error {
	return &errSetProviderConfig{err: err}
}

// ResourceServerAdapter Adapter for a tfprotov6.ResourceServer removing the error return type from all methods.
type ResourceServerAdapter interface {
	ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse)
	UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse)
	ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse)
	PlanResourceChange(ctx context.Context, req PlanResourceChangeRequest, res *PlanResourceChangeResponse)
	ApplyResourceChange(ctx context.Context, req ApplyResourceChangeRequest, res *ApplyResourceChangeResponse)
	ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse)
}

// Resource represents a Terraform resource.
type Resource interface {
	ResourceServerAdapter
	Name() string
	Schema() *tfprotov6.Schema
	SetProviderConfig(val tftypes.Value) error
}

// RouterOpt is a functional option for the router constructor.
type RouterOpt func(Router) Router

// New takes zero or more functional options and returns a new Router.
func New(opts ...RouterOpt) Router {
	r := newRouter()
	for _, opt := range opts {
		r = opt(r)
	}

	return r
}

// RegisterResource is a functional option that register a new Resource with
// the Router.
func RegisterResource(resource Resource) func(Router) Router {
	return func(router Router) Router {
		router.resources[resource.Name()] = resource

		return router
	}
}

func newRouter() Router {
	return Router{
		resources: map[string]Resource{},
	}
}

// Router routes the requests the resource servers.
type Router struct {
	resources map[string]Resource
}

// ValidateResourceConfig validates the resource's config.
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

// UpgradeResourceState upgrades the state when migrating from an old version to a new version.
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

// ReadResource refreshes the resource's state.
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

// ImportResourceState fetches the resource from an ID and adds it to the state.
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

// Schemas returns the data router schemas.
func (r Router) Schemas() map[string]*tfprotov6.Schema {
	schemas := map[string]*tfprotov6.Schema{}
	for name, resource := range r.resources {
		schemas[name] = resource.Schema()
	}

	return schemas
}
