// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resourcerouter

import (
	"context"
	"fmt"

	"github.com/hashicorp/enos-provider/internal/diags"
	"github.com/hashicorp/enos-provider/internal/server/state"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// ApplyResourceChangeRequest An adapter type that mirrors the type tfproto6.ApplyResourceChangeRequest
// exposing the prior and planned states as unmarshalled tftypes.Value values rather than
// tfprotov6.DynamicValue values.
type ApplyResourceChangeRequest struct {
	TypeName       string
	PriorState     tftypes.Value
	PlannedState   tftypes.Value
	Config         *tfprotov6.DynamicValue
	PlannedPrivate []byte
	ProviderMeta   *tfprotov6.DynamicValue
}

// IsDelete if true this request represents a delete request.
func (a *ApplyResourceChangeRequest) IsDelete() bool {
	return !a.PriorState.IsNull() && a.PlannedState.IsNull()
}

// IsCreate if true this request represents a create request.
func (a *ApplyResourceChangeRequest) IsCreate() bool {
	return a.PriorState.IsNull() && !a.PlannedState.IsNull()
}

// IsUpdate if true this request represents an update request.
func (a *ApplyResourceChangeRequest) IsUpdate() bool {
	return !a.IsDelete() && !a.IsCreate()
}

// fromTFProto6 converts a tfproto6 request to the adapter request type.
func (a *ApplyResourceChangeRequest) fromTFProto6(req *tfprotov6.ApplyResourceChangeRequest, tfType tftypes.Type) error {
	priorState, err := req.PriorState.Unmarshal(tfType)
	if err != nil {
		return fmt.Errorf("failed to unmarshal prior state, due to: %w", err)
	}

	plannedState, err := req.PlannedState.Unmarshal(tfType)
	if err != nil {
		return fmt.Errorf("failed to unmarshal planned state, due to: %w", err)
	}

	a.TypeName = req.TypeName
	a.PriorState = priorState
	a.PlannedState = plannedState
	a.Config = req.Config
	a.PlannedPrivate = req.PlannedPrivate
	a.ProviderMeta = req.ProviderMeta

	return nil
}

// ApplyResourceChangeResponse An adapter type that mirrors the type tfproto6.ApplyResourceChangeResponse
// exposing the resultant new state as an unmarshalled state.State type rather than a marshalled
// tfprotov6.DynamicValue.
type ApplyResourceChangeResponse struct {
	NewState                    state.State
	Private                     []byte
	Diagnostics                 []*tfprotov6.Diagnostic
	UnsafeToUseLegacyTypeSystem bool
}

// ToTFProto6Response Converts the response to a tfproto6 response type.
func (a ApplyResourceChangeResponse) ToTFProto6Response(isDelete bool) *tfprotov6.ApplyResourceChangeResponse {
	resp := &tfprotov6.ApplyResourceChangeResponse{
		Private:                     a.Private,
		Diagnostics:                 a.Diagnostics,
		UnsafeToUseLegacyTypeSystem: a.UnsafeToUseLegacyTypeSystem,
	}

	if !diags.HasErrors(a.Diagnostics) {
		val, err := marshalApply(a.NewState, isDelete)
		if err != nil {
			resp.Diagnostics = append(resp.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		} else {
			resp.NewState = val
		}
	}

	return resp
}

// ApplyResourceChange applies the newly planned resource state and executes any configured failure
// handlers.
func (r Router) ApplyResourceChange(ctx context.Context, req *tfprotov6.ApplyResourceChangeRequest, providerConfig tftypes.Value) (*tfprotov6.ApplyResourceChangeResponse, error) {
	resource, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := resource.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	res := &ApplyResourceChangeResponse{
		Diagnostics: []*tfprotov6.Diagnostic{},
	}
	request := &ApplyResourceChangeRequest{}

	err = request.fromTFProto6(req, resource.Schema().ValueType())
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
	} else {
		resource.ApplyResourceChange(ctx, *request, res)
	}

	if errDiag := diags.GetErrorDiagnostic(res.Diagnostics); errDiag != nil {
		res.NewState.HandleFailure(ctx, errDiag, providerConfig)
	}

	return res.ToTFProto6Response(request.IsDelete()), nil
}

func marshalApply(serializable state.Serializable, isDelete bool) (*tfprotov6.DynamicValue, error) {
	if isDelete {
		return state.MarshalDelete(serializable)
	}

	return state.Marshal(serializable)
}
