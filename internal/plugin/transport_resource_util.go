package plugin

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

var transportUtil = &transportResourceUtil{}

// transportResourceUtil is a container for helper functions that are useful
// when building a plugin resource that uses an embedded transport.
type transportResourceUtil struct {
}

// ValidateResourceTypeConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (t *transportResourceUtil) ValidateResourceTypeConfig(ctx context.Context, state StateWithTransport, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	res := &tfprotov5.ValidateResourceTypeConfigResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	err := unmarshal(state, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	return res, err
}

// DefaultUpgradeResourceState is the request Terraform sends when it wants to
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
func (t *transportResourceUtil) UpgradeResourceState(ctx context.Context, state StateWithTransport, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
	res := &tfprotov5.UpgradeResourceStateResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	switch req.Version {
	case 1:
		// 1. unmarshal the raw state against the type that maps to the raw state
		// version. As this is version 1 and we're on version 1 we can use the
		// current state type.
		rawStateValues, err := req.RawState.Unmarshal(state.Terraform5Type())
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(wrapErrWithDiagnostics(
				err,
				"upgrade error",
				"unable to map version 1 to the current state",
			)))
			return res, err
		}

		// 2. Since we're on version one we can pass the same values in without
		// doing a transform.

		// 3. Upgrade the current state with the new values, or in this case,
		// the raw values.
		res.UpgradedState, err = upgradeState(state, rawStateValues)

		return res, err
	default:
		err := newErrWithDiagnostics(
			"Unexpected state version",
			"The provider doesn't know how to upgrade from the current state version",
		)
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (t *transportResourceUtil) ReadResource(ctx context.Context, state StateWithTransport, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	res := &tfprotov5.ReadResourceResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	err := unmarshal(state, req.CurrentState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	err = state.EmbeddedTransport().FromPrivate(req.Private)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	res.NewState, err = marshal(state)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	res.Private, err = state.EmbeddedTransport().ToPrivate()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	return res, err
}

// PlanUnmarshalVerifyAndBuildTransport is a helper method that unmarshals
// a request into prior and proposed states, builds a transport client,
// verifies it, and returns the new transport.
func (t *transportResourceUtil) PlanUnmarshalVerifyAndBuildTransport(ctx context.Context, prior StateWithTransport, proposed StateWithTransport, resource ResourceWithProviderConfig, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, *embeddedTransportV1, error) {
	res := &tfprotov5.PlanResourceChangeResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, nil, ctx.Err()
	default:
	}

	providerConfig, err := resource.GetProviderConfig()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, nil, err
	}

	transport, err := providerConfig.Transport.Copy()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, nil, err
	}

	err = unmarshal(prior, req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, transport, err
	}

	priorTransport := prior.EmbeddedTransport()
	err = priorTransport.FromPrivate(req.PriorPrivate)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, transport, err
	}

	err = unmarshal(proposed, req.ProposedNewState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, transport, err
	}

	// Use any provider configuration
	proposedTransport := proposed.EmbeddedTransport()
	err = proposedTransport.MergeInto(transport)
	if err != nil {
		err = wrapErrWithDiagnostics(err,
			"invalid configuration", "failed to merge resource and provider transport configuration", "transport", "ssh",
		)
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, transport, err
	}

	res.RequiresReplace = transportReplacedAttributePaths(priorTransport, proposedTransport)

	return res, transport, err
}

// PlanMarshalPlannedState marshals a proposed state and transport into a plan response
func (t *transportResourceUtil) PlanMarshalPlannedState(ctx context.Context, res *tfprotov5.PlanResourceChangeResponse, proposed StateWithTransport, transport *embeddedTransportV1) error {
	var err error

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return ctx.Err()
	default:
	}

	res.PlannedState, err = marshal(proposed)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return err
	}

	res.PlannedPrivate, err = transport.ToPrivate()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return err
	}

	return nil
}

// ApplyUnmarshalState is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (t *transportResourceUtil) ApplyUnmarshalState(ctx context.Context, prior StateWithTransport, planned StateWithTransport, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
	res := &tfprotov5.ApplyResourceChangeResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	err := unmarshal(planned, req.PlannedState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	transport := planned.EmbeddedTransport()
	err = transport.FromPrivate(req.PlannedPrivate)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	err = unmarshal(prior, req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	return res, nil
}

// ApplyValidatePlannedAndBuildClient takes the planned state and provider transport,
// validates them, and returns a new SSH transport client.
func (t *transportResourceUtil) ApplyValidatePlannedAndBuildTransport(ctx context.Context, res *tfprotov5.ApplyResourceChangeResponse, planned StateWithTransport, resource ResourceWithProviderConfig) (*embeddedTransportV1, error) {
	providerConfig, err := resource.GetProviderConfig()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return nil, err
	}

	// Always work with a copy of the provider config so that we don't race
	// for the pointer.
	providerConfig, err = providerConfig.Copy()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return nil, err
	}

	transport := providerConfig.Transport

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return transport, ctx.Err()
	default:
	}

	etP := planned.EmbeddedTransport()
	et, err := etP.Copy()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return transport, err
	}

	err = et.MergeInto(transport)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return transport, err
	}

	err = transport.Validate(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return transport, err
	}

	err = planned.Validate(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return transport, err
	}

	return transport, nil
}

// ApplyMarshalNewState takes the planned state and transport and marshal it
// into the new state
func (t *transportResourceUtil) ApplyMarshalNewState(ctx context.Context, res *tfprotov5.ApplyResourceChangeResponse, planned StateWithTransport, transport *embeddedTransportV1) error {
	var err error

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return ctx.Err()
	default:
	}

	res.NewState, err = marshal(planned)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return err
	}

	res.Private, err = transport.ToPrivate()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return err
	}

	return nil
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
//
// Importing a enos resources doesn't make a lot of sense but we have to support the
// function regardless.
func (t *transportResourceUtil) ImportResourceState(ctx context.Context, state StateWithTransport, req *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
	res := &tfprotov5.ImportResourceStateResponse{
		ImportedResources: []*tfprotov5.ImportedResource{},
		Diagnostics:       []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	importState, err := marshal(state)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}
	res.ImportedResources = append(res.ImportedResources, &tfprotov5.ImportedResource{
		TypeName: req.TypeName,
		State:    importState,
	})

	return res, err
}
