package resourcerouter

import (
	"context"
	"fmt"

	"github.com/hashicorp/enos-provider/internal/diags"
	"github.com/hashicorp/enos-provider/internal/server/state"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// PlanResourceChangeRequest An adapter type that mirrors the type tfproto6.PlanResourceChangeRequest
// exposing the prior and proposed states as unmarshalled tftypes.Value values rather than
// tfprotov6.DynamicValue values.
type PlanResourceChangeRequest struct {
	TypeName         string
	PriorState       tftypes.Value
	ProposedNewState tftypes.Value
	Config           *tfprotov6.DynamicValue
	PriorPrivate     []byte
	ProviderMeta     *tfprotov6.DynamicValue
}

func (p *PlanResourceChangeRequest) fromTFProto6(req *tfprotov6.PlanResourceChangeRequest, tfType tftypes.Type) error {
	priorState, err := req.PriorState.Unmarshal(tfType)
	if err != nil {
		return fmt.Errorf("failed to unmarshal prior state, due to: %w", err)
	}

	proposedNewState, err := req.ProposedNewState.Unmarshal(tfType)
	if err != nil {
		return fmt.Errorf("failed to unmarshal proposed state, due to: %w", err)
	}

	p.TypeName = req.TypeName
	p.PriorState = priorState
	p.ProposedNewState = proposedNewState
	p.Config = req.Config
	p.PriorPrivate = req.PriorPrivate
	p.ProviderMeta = req.ProviderMeta

	return nil
}

// PlanResourceChangeResponse An adapter type that mirrors the type tfproto6.PlanResourceChangeResponse
// exposing the resultant plan as an unmarshalled state.State type rather than a marshalled
// tfprotov6.DynamicValue.
type PlanResourceChangeResponse struct {
	PlannedState                state.State
	RequiresReplace             []*tftypes.AttributePath
	PlannedPrivate              []byte
	Diagnostics                 []*tfprotov6.Diagnostic
	UnsafeToUseLegacyTypeSystem bool
}

// ToTFProto6Response Converts the response to a tfproto6 response type. Adds debug information to
// the diagnostic if the plan request failed.
func (p PlanResourceChangeResponse) ToTFProto6Response() *tfprotov6.PlanResourceChangeResponse {
	resp := &tfprotov6.PlanResourceChangeResponse{
		RequiresReplace:             p.RequiresReplace,
		PlannedPrivate:              p.PlannedPrivate,
		Diagnostics:                 p.Diagnostics,
		UnsafeToUseLegacyTypeSystem: p.UnsafeToUseLegacyTypeSystem,
	}

	if !diags.HasErrors(p.Diagnostics) {
		val, err := state.Marshal(p.PlannedState)
		if err != nil {
			resp.Diagnostics = append(resp.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		} else {
			resp.PlannedState = val
		}
	}

	return resp
}

// PlanResourceChange proposes a new resource state.
func (r Router) PlanResourceChange(ctx context.Context, req *tfprotov6.PlanResourceChangeRequest, providerConfig tftypes.Value) (*tfprotov6.PlanResourceChangeResponse, error) {
	resource, ok := r.resources[req.TypeName]
	if !ok {
		return nil, errUnsupportedResource(req.TypeName)
	}

	err := resource.SetProviderConfig(providerConfig)
	if err != nil {
		return nil, newErrSetProviderConfig(err)
	}

	res := &PlanResourceChangeResponse{
		Diagnostics: []*tfprotov6.Diagnostic{},
	}
	request := &PlanResourceChangeRequest{}

	if err = request.fromTFProto6(req, resource.Schema().ValueType()); err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
	} else {
		resource.PlanResourceChange(ctx, *request, res)
	}

	if errDiag := diags.GetErrorDiagnostic(res.Diagnostics); errDiag != nil {
		res.PlannedState.HandleFailure(ctx, errDiag, providerConfig)
	}

	return res.ToTFProto6Response(), nil
}
