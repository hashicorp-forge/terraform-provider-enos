package plugin

import (
	"context"

	tfile "github.com/hashicorp/enos-provider/internal/transport/file"

	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tftypes"
)

type file struct{}

var _ resourcerouter.Resource = (*file)(nil)

type fileStateV1 struct {
	ID        string
	Src       string
	Dst       string
	Transport *embeddedTransportV1
}

var _ State = (*fileStateV1)(nil)

func newFile() *file {
	return &file{}
}

func newFileState() *fileStateV1 {
	return &fileStateV1{
		Transport: newEmbeddedTransport(),
	}
}

func (f *file) Name() string {
	return "enos_file"
}

func (f *file) Schema() *tfprotov5.Schema {
	return newFileState().Schema()
}

// ValidateResourceTypeConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (f *file) ValidateResourceTypeConfig(ctx context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	res := &tfprotov5.ValidateResourceTypeConfigResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	newState := newFileState()
	err := unmarshal(newState, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	return res, err
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
func (f *file) UpgradeResourceState(ctx context.Context, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
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
		newState := newFileState()
		rawStateValues, err := req.RawState.Unmarshal(newState.Terraform5Type())
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
		res.UpgradedState, err = upgradeState(newState, rawStateValues)

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
func (f *file) ReadResource(ctx context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	res := &tfprotov5.ReadResourceResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	// unmarshal and re-marshal the state to add default fields
	newState := newFileState()
	err := unmarshal(newState, req.CurrentState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	res.NewState, err = marshal(newState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	return res, err
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (f *file) PlanResourceChange(ctx context.Context, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, error) {
	res := &tfprotov5.PlanResourceChangeResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	proposedState := newFileState()
	err := unmarshal(proposedState, req.ProposedNewState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	res.PlannedState, err = marshal(proposedState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	return res, err
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (f *file) ApplyResourceChange(ctx context.Context, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
	res := &tfprotov5.ApplyResourceChangeResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	addDiagnostic := func(res *tfprotov5.ApplyResourceChangeResponse, err error) (*tfprotov5.ApplyResourceChangeResponse, error) {
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		}

		return res, err
	}

	select {
	case <-ctx.Done():
		return addDiagnostic(res, ctx.Err())
	default:
	}

	plannedState := newFileState()
	err := unmarshal(plannedState, req.PlannedState)
	if err != nil {
		return addDiagnostic(res, err)
	}

	// If our planned state does not have a source then the planned state terrafom
	// gave us is null, therefore we're supposed to delete the resource.
	if plannedState.Src == "" {
		// Currently this is a no-op. If we decide to make file delete destructive
		// we'll need to remove the remote file.
		res.NewState, err = marshalDelete(plannedState)

		return res, err
	}

	err = plannedState.Validate(ctx)
	if err != nil {
		return addDiagnostic(res, err)
	}

	// Get the file's SHA256 sum, which is the file's ID.
	sha256, err := tfile.SHA256(plannedState.Src)
	if err != nil {
		return addDiagnostic(res, wrapErrWithDiagnostics(
			err, "invalid configuration", "unable to obtain source file SHA256 sum", "source",
		))
	}
	plannedState.ID = sha256

	ssh, err := plannedState.Transport.Client(ctx)
	if err != nil {
		return addDiagnostic(res, err)
	}

	src, err := tfile.Open(plannedState.Src)
	if err != nil {
		return addDiagnostic(res, wrapErrWithDiagnostics(
			err, "invalid configuration", "unable to open source file", "source",
		))
	}

	priorState := newFileState()
	err = unmarshal(priorState, req.PriorState)
	if err != nil {
		return addDiagnostic(res, err)
	}

	// If our priorState ID is blank then we're creating the file
	if priorState.ID == "" {
		err = ssh.Copy(ctx, src, plannedState.Dst)
		if err != nil {
			return addDiagnostic(res, err)
		}
	} else if priorState.ID != "" && priorState.ID != sha256 {
		// If our priorState.ID matches the sum of the file then it's an update
		// and a no-op. If they have diverged then upload the new file.
		err = ssh.Copy(ctx, src, plannedState.Dst)
		if err != nil {
			return addDiagnostic(res, err)
		}
	}

	res.NewState, err = marshal(plannedState)
	if err != nil {
		return addDiagnostic(res, err)
	}

	return res, err
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
//
// Importing a file doesn't make a lot of sense but we have to support the
// function regardless. As our only interface is a string ID, supporting this
// without provider level transport configuration would be absurdly difficult.
// Until then this will simply be a no-op. If/When we implement that behavior
// we could probably create use an identier that combines the source and
// destination to import a file.
func (f *file) ImportResourceState(ctx context.Context, req *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
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

	newState := newFileState()
	state, err := marshal(newState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}
	res.ImportedResources = append(res.ImportedResources, &tfprotov5.ImportedResource{
		TypeName: req.TypeName,
		State:    state,
	})

	return res, err
}

// Schema is the file states Terraform schema.
func (fs *fileStateV1) Schema() *tfprotov5.Schema {
	return &tfprotov5.Schema{
		Version: 1,
		Block: &tfprotov5.SchemaBlock{
			Attributes: []*tfprotov5.SchemaAttribute{
				{
					Name:     "id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "source",
					Type:     tftypes.String,
					Required: true,
				},
				{
					Name:     "destination",
					Type:     tftypes.String,
					Required: true,
				},
				fs.Transport.SchemaAttributeTransport(),
			},
		},
	}
}

// Validate validates the configuration. This will validate the source file
// exists and that the transport configuration is valid.
func (fs *fileStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if fs.Src == "" {
		return newErrWithDiagnostics("invalid configuration", "you must provide the source location", "source")
	}

	_, err := tfile.Open(fs.Src)
	if err != nil {
		return newErrWithDiagnostics("invalid configuration", "unable to open source file", "source")
	}

	if fs.Dst == "" {
		return newErrWithDiagnostics("invalid configuration", "you must provide the destination location", "destination")
	}

	return fs.Transport.Validate(ctx)
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (fs *fileStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id":          &fs.ID,
		"source":      &fs.Src,
		"destination": &fs.Dst,
	})
	if err != nil {
		return err
	}

	if !vals["transport"].IsKnown() {
		return nil
	}

	return fs.Transport.FromTerraform5Value(vals["transport"])
}

// Terraform5Type is the file state tftypes.Type.
func (fs *fileStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":          tftypes.String,
		"source":      tftypes.String,
		"destination": tftypes.String,
		"transport":   fs.Transport.Terraform5Type(),
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (fs *fileStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(fs.Terraform5Type(), map[string]tftypes.Value{
		"id":          stringValue(fs.ID),
		"source":      stringValue(fs.Src),
		"destination": stringValue(fs.Dst),
		"transport":   fs.Transport.Terraform5Value(),
	})
}
