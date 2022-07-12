package plugin

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var transportUtil = &transportResourceUtil{}

// transportResourceUtil is a container for helper functions that are useful
// when building a plugin resource that uses an embedded transport.
type transportResourceUtil struct{}

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (t *transportResourceUtil) ValidateResourceConfig(ctx context.Context, state Serializable, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	if err := unmarshal(state, req.Config); err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
	}
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
//
// Upgrading the resource state generally goes as follows:
//
//   1. Unmarshal the RawState to the corresponding tftypes.Value that matches
//     schema version of the state we're upgrading from.
//   2. Create a new tftypes.Value for the current state and migrate the old
//    values to the new values.
//   3. Upgrade the existing state with the new values and return the marshaled
//    version of the current upgraded state.
//
func (t *transportResourceUtil) UpgradeResourceState(ctx context.Context, state Serializable, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
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
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		}
	default:
		// TODO: shouldn't this raise an error?
	}
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (t *transportResourceUtil) ReadResource(ctx context.Context, state StateWithTransport, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	err := unmarshal(state, req.CurrentState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	res.NewState, err = marshal(state)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	if err != nil {
		res.Diagnostics = append(res.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Read Resource Error",
			Detail:   fmt.Sprintf("Failed marshal embedded transport due to: %s", err.Error()),
		})
	}
}

// PlanUnmarshalVerifyAndBuildTransport is a helper method that unmarshals
// a request into prior and proposed states, builds a transport client,
// verifies it, and returns the new transport.
func (t *transportResourceUtil) PlanUnmarshalVerifyAndBuildTransport(ctx context.Context, prior StateWithTransport, proposed StateWithTransport, resource ResourceWithProviderConfig, req tfprotov6.PlanResourceChangeRequest, res *tfprotov6.PlanResourceChangeResponse) *embeddedTransportV1 {
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

	err = unmarshal(prior, req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return nil
	}

	err = unmarshal(proposed, req.ProposedNewState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return nil
	}

	proposedTransport, err := proposed.EmbeddedTransport().Copy()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Plan Error",
			Detail:   fmt.Sprintf("Failed to get proposed transport config, due to: %s", err.Error()),
		})
		return nil
	}
	err = proposedTransport.ApplyDefaults(providerConfig.Transport)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return nil
	}

	res.RequiresReplace = prior.EmbeddedTransport().transportReplacedAttributePaths(proposedTransport)

	return proposedTransport
}

// PlanMarshalPlannedState marshals a proposed state and transport into a plan response
func (t *transportResourceUtil) PlanMarshalPlannedState(ctx context.Context, res *tfprotov6.PlanResourceChangeResponse, proposed StateWithTransport, transport *embeddedTransportV1) {
	var err error

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	res.PlannedState, err = marshal(proposed)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}
}

// ApplyUnmarshalState is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (t *transportResourceUtil) ApplyUnmarshalState(ctx context.Context, prior StateWithTransport, planned StateWithTransport, req tfprotov6.ApplyResourceChangeRequest, res *tfprotov6.ApplyResourceChangeResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	err := unmarshal(planned, req.PlannedState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	if err != nil {
		res.Diagnostics = append(res.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Plan Error",
			Detail:   fmt.Sprintf("Failed unmarshal prior transport, due to: %s", err.Error()),
		})
		return
	}

	err = unmarshal(prior, req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
	}
}

// ApplyValidatePlannedAndBuildTransport takes the planned state and provider transport,
// validates them, and returns a new embedded transport that can be used to create a transport client.
func (t *transportResourceUtil) ApplyValidatePlannedAndBuildTransport(ctx context.Context, planned StateWithTransport, resource ResourceWithProviderConfig, res *tfprotov6.ApplyResourceChangeResponse) *embeddedTransportV1 {
	providerConfig, err := resource.GetProviderConfig()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Apply Error",
			Detail:   fmt.Sprintf("Failed to get provider config, due to: %s", err.Error()),
		})
		return nil
	}

	// Always work with a copy of the provider config so that we don't race
	// for the pointer.
	providerConfig, err = providerConfig.Copy()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Apply Error",
			Detail:   fmt.Sprintf("Failed to copy provider config, due to: %s", err.Error()),
		})
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
		res.Diagnostics = append(res.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityError,
			Summary:  "Apply Error",
			Detail:   fmt.Sprintf("Failed to copy embedded transport, due to: %s", err.Error()),
		})
		return nil
	}

	err = et.ApplyDefaults(providerConfig.Transport)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return nil
	}

	err = et.Validate(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return nil
	}

	err = planned.Validate(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return nil
	}

	return et
}

// ApplyMarshalNewState takes the planned state and transport and marshal it
// into the new state
func (t *transportResourceUtil) ApplyMarshalNewState(ctx context.Context, res *tfprotov6.ApplyResourceChangeResponse, planned StateWithTransport, transport *embeddedTransportV1) {
	var err error

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	res.NewState, err = marshal(planned)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
//
// Importing a enos resources doesn't make a lot of sense but we have to support the
// function regardless.
func (t *transportResourceUtil) ImportResourceState(ctx context.Context, state Serializable, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	importState, err := marshal(state)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	res.ImportedResources = append(res.ImportedResources, &tfprotov6.ImportedResource{
		TypeName: req.TypeName,
		State:    importState,
	})
}
