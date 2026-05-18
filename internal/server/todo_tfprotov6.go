// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package server

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// MoveResourceState is called when Terraform is asked to change a resource type for an existing resource.
// This provider does not support moving resource state between types.
func (s Server) MoveResourceState(ctx context.Context, req *tfprotov6.MoveResourceStateRequest) (*tfprotov6.MoveResourceStateResponse, error) {
	return &tfprotov6.MoveResourceStateResponse{
		Diagnostics: []*tfprotov6.Diagnostic{
			{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "Move Resource State Not Supported",
				Detail:   "This provider does not support moving resource state between types",
			},
		},
	}, nil
}

// UpgradeResourceIdentity is called when Terraform has encountered a resource with an identity state
// in a schema that doesn't match the schema's current version.
// This provider does not support resource identity upgrades.
func (s Server) UpgradeResourceIdentity(ctx context.Context, req *tfprotov6.UpgradeResourceIdentityRequest) (*tfprotov6.UpgradeResourceIdentityResponse, error) {
	return &tfprotov6.UpgradeResourceIdentityResponse{
		Diagnostics: []*tfprotov6.Diagnostic{
			{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "Upgrade Resource Identity Not Supported",
				Detail:   "This provider does not support resource identity upgrades",
			},
		},
	}, nil
}

// GenerateResourceConfig is called when Terraform wants to generate a resource configuration
// for importing to a resource address that doesn't exist yet.
// This provider does not support generating resource configurations.
func (s Server) GenerateResourceConfig(ctx context.Context, req *tfprotov6.GenerateResourceConfigRequest) (*tfprotov6.GenerateResourceConfigResponse, error) {
	return &tfprotov6.GenerateResourceConfigResponse{
		Diagnostics: []*tfprotov6.Diagnostic{
			{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "Generate Resource Config Not Supported",
				Detail:   "This provider does not support generating resource configurations",
			},
		},
	}, nil
}

// GetResourceIdentitySchemas returns the resource identity schemas for the provider.
// This provider does not use resource identity features, so we return an empty response.
func (s Server) GetResourceIdentitySchemas(ctx context.Context, req *tfprotov6.GetResourceIdentitySchemasRequest) (*tfprotov6.GetResourceIdentitySchemasResponse, error) {
	return &tfprotov6.GetResourceIdentitySchemasResponse{}, nil
}

// CallFunction executes the logic of a function referenced in the configuration.
// This provider does not support functions.
func (s Server) CallFunction(ctx context.Context, req *tfprotov6.CallFunctionRequest) (*tfprotov6.CallFunctionResponse, error) {
	return &tfprotov6.CallFunctionResponse{
		Error: &tfprotov6.FunctionError{
			Text: "provider functions are not supported",
		},
	}, nil
}

// GetFunctions returns the functions supported by the provider.
// This provider does not support functions.
func (s Server) GetFunctions(ctx context.Context, req *tfprotov6.GetFunctionsRequest) (*tfprotov6.GetFunctionsResponse, error) {
	return &tfprotov6.GetFunctionsResponse{
		Functions: map[string]*tfprotov6.Function{},
	}, nil
}

// ValidateEphemeralResourceConfig validates an ephemeral resource configuration.
// This provider does not support ephemeral resources.
func (s Server) ValidateEphemeralResourceConfig(ctx context.Context, req *tfprotov6.ValidateEphemeralResourceConfigRequest) (*tfprotov6.ValidateEphemeralResourceConfigResponse, error) {
	return &tfprotov6.ValidateEphemeralResourceConfigResponse{
		Diagnostics: []*tfprotov6.Diagnostic{
			{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "Ephemeral Resources Not Supported",
				Detail:   "This provider does not support ephemeral resources",
			},
		},
	}, nil
}

// OpenEphemeralResource opens an ephemeral resource.
// This provider does not support ephemeral resources.
func (s Server) OpenEphemeralResource(ctx context.Context, req *tfprotov6.OpenEphemeralResourceRequest) (*tfprotov6.OpenEphemeralResourceResponse, error) {
	return &tfprotov6.OpenEphemeralResourceResponse{
		Diagnostics: []*tfprotov6.Diagnostic{
			{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "Ephemeral Resources Not Supported",
				Detail:   "This provider does not support ephemeral resources",
			},
		},
	}, nil
}

// RenewEphemeralResource renews an ephemeral resource.
// This provider does not support ephemeral resources.
func (s Server) RenewEphemeralResource(ctx context.Context, req *tfprotov6.RenewEphemeralResourceRequest) (*tfprotov6.RenewEphemeralResourceResponse, error) {
	return &tfprotov6.RenewEphemeralResourceResponse{
		Diagnostics: []*tfprotov6.Diagnostic{
			{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "Ephemeral Resources Not Supported",
				Detail:   "This provider does not support ephemeral resources",
			},
		},
	}, nil
}

// CloseEphemeralResource closes an ephemeral resource.
// This provider does not support ephemeral resources.
func (s Server) CloseEphemeralResource(ctx context.Context, req *tfprotov6.CloseEphemeralResourceRequest) (*tfprotov6.CloseEphemeralResourceResponse, error) {
	return &tfprotov6.CloseEphemeralResourceResponse{
		Diagnostics: []*tfprotov6.Diagnostic{
			{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "Ephemeral Resources Not Supported",
				Detail:   "This provider does not support ephemeral resources",
			},
		},
	}, nil
}
