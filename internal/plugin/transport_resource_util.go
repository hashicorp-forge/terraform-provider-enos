package plugin

import (
	"context"
	"fmt"

	"github.com/hashicorp/enos-provider/internal/diags"
	resource "github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/enos-provider/internal/server/state"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var transportUtil = &transportResourceUtil{}

// transportResourceUtil is a container for helper functions that are useful
// when building a plugin resource that uses an embedded transport.
type transportResourceUtil struct{}

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (t *transportResourceUtil) ValidateResourceConfig(ctx context.Context, state state.Serializable, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	defer func() {
		// The library we use to convert the wire config to tftypes can panic. We'll recover
		// here to bubble up those errors as diagnostics instead of just panicking and leaving
		// Terraform hanging.
		if pan := recover(); pan != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
				"Serialization Error", fmt.Errorf(
					"%v state: %+v, config:%+v ", pan, state, req.Config),
			))
		}
	}()

	if err := unmarshal(state, req.Config); err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
	}
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
//
// Upgrading the resource state generally goes as follows:
//
//  1. Unmarshal the RawState to the corresponding tftypes.Value that matches
//     schema version of the state we're upgrading from.
//  2. Create a new tftypes.Value for the current state and migrate the old
//     values to the new values.
//  3. Upgrade the existing state with the new values and return the marshaled
//     version of the current upgraded state.
func (t *transportResourceUtil) UpgradeResourceState(ctx context.Context, state state.Serializable, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	switch req.Version {
	case 1:
		// 1. unmarshal the raw state against the type that maps to the raw state
		// version. As this is version 1 and we're on version 1 we can use the
		// current state type.
		rawStateValues, err := req.RawState.Unmarshal(state.Terraform5Type())
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, &tfprotov6.Diagnostic{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "upgrade error",
				Detail:   fmt.Sprintf("unable to map version 1 to the current state, due to: %s", err.Error()),
			})

			return
		}

		// 2. Since we're on version one we can pass the same values in without
		// doing a transform.

		// 3. Upgrade the current state with the new values, or in this case,
		// the raw values.
		res.UpgradedState, err = upgradeState(state, rawStateValues)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Upgrade State Error", err))
		}
	default:
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
			"Upgrade State Error",
			fmt.Errorf("the provider doesn't know how to upgrade from the current state version"),
		))
	}
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (t *transportResourceUtil) ReadResource(ctx context.Context, serializable state.Serializable, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	defer func() {
		// The library we use to convert the wire config to tftypes can panic. We'll recover
		// here to bubble up those errors as diagnostics instead of just panicking and leaving
		// Terraform hanging.
		if pan := recover(); pan != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
				"Serialization Error", fmt.Errorf(
					"%v new state: %+v, current state:%+v ", pan, res.NewState, req.CurrentState),
			))
		}
	}()

	err := unmarshal(serializable, req.CurrentState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	res.NewState, err = state.Marshal(serializable)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}
}

// PlanUnmarshalVerifyAndBuildTransport is a helper method that unmarshals
// a request into prior and proposed states, builds a transport client,
// verifies it, and returns the new transport.
func (t *transportResourceUtil) PlanUnmarshalVerifyAndBuildTransport(
	ctx context.Context,
	prior, proposed StateWithTransport,
	resource ResourceWithProviderConfig,
	req resource.PlanResourceChangeRequest,
	res *resource.PlanResourceChangeResponse,
) *embeddedTransportV1 {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return nil
	default:
	}

	providerConfig, err := resource.GetProviderConfig()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Plan Error",
			Detail:   fmt.Sprintf("Failed to get provider config, due to: %s", err.Error()),
		})

		return nil
	}

	err = prior.FromTerraform5Value(req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return nil
	}

	err = proposed.FromTerraform5Value(req.ProposedNewState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return nil
	}

	proposedTransport, err := proposed.EmbeddedTransport().Copy()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
			"Transport Error",
			fmt.Errorf("failed to get proposed transport config, due to: %w", err),
		))

		return nil
	}
	configuredTransport, err := proposedTransport.ApplyDefaults(providerConfig.Transport)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Transport Error", err))
		return nil
	}
	proposed.EmbeddedTransport().setResolvedTransport(configuredTransport)

	res.RequiresReplace = prior.EmbeddedTransport().transportReplacedAttributePaths(proposedTransport)

	return proposedTransport
}

// ApplyUnmarshalState is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (t *transportResourceUtil) ApplyUnmarshalState(ctx context.Context, prior, planned StateWithTransport, req resource.ApplyResourceChangeRequest, res *resource.ApplyResourceChangeResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	err := planned.FromTerraform5Value(req.PlannedState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
			"Serialization Error",
			fmt.Errorf("failed unmarshal planned state, due to: %s", err),
		))

		return
	}

	err = prior.FromTerraform5Value(req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
			"Serialization Error",
			fmt.Errorf("failed unmarshal prior state, due to: %s", err),
		))
	}
}

// ApplyValidatePlannedAndBuildTransport takes the planned state and provider transport,
// validates them, and returns a new embedded transport that can be used to create a transport client.
func (t *transportResourceUtil) ApplyValidatePlannedAndBuildTransport(ctx context.Context, planned StateWithTransport, resource ResourceWithProviderConfig, res *resource.ApplyResourceChangeResponse) *embeddedTransportV1 {
	providerConfig, err := resource.GetProviderConfig()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
			"Apply Error",
			fmt.Errorf("failed to get provider config, due to: %s", err),
		))

		return nil
	}

	// Always work with a copy of the provider config so that we don't race
	// for the pointer.
	providerConfig, err = providerConfig.Copy()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
			"Apply Error",
			fmt.Errorf("failed to copy provider config, due to: %s", err),
		))

		return nil
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return nil
	default:
	}

	etP := planned.EmbeddedTransport()
	et, err := etP.Copy()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
			"Apply Error",
			fmt.Errorf("failed to copy embedded transport, due to: %s", err),
		))

		return nil
	}

	configuredTransport, err := et.ApplyDefaults(providerConfig.Transport)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
			"Transport Error",
			fmt.Errorf("failed to apply transport defaults, due to: %w", err),
		))

		return nil
	}
	etP.setResolvedTransport(configuredTransport)

	err = et.Validate(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Validation Error", err))
		return nil
	}

	err = planned.Validate(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Validation Error", err))
		return nil
	}

	return et
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
//
// Importing a enos resources doesn't make a lot of sense but we have to support the
// function regardless.
func (t *transportResourceUtil) ImportResourceState(ctx context.Context, serializable state.Serializable, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	importState, err := state.Marshal(serializable)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	res.ImportedResources = append(res.ImportedResources, &tfprotov6.ImportedResource{
		TypeName: req.TypeName,
		State:    importState,
	})
}
