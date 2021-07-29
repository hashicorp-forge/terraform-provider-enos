package plugin

import (
	"context"
	"sync"

	"github.com/hashicorp/enos-provider/internal/flightcontrol/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"

	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type file struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resourcerouter.Resource = (*file)(nil)

type fileStateV1 struct {
	ID        *tfString
	Src       *tfString
	Dst       *tfString
	Content   *tfString
	Sum       *tfString
	TmpDir    *tfString
	Chmod     *tfString
	Chown     *tfString
	Transport *embeddedTransportV1
}

var _ State = (*fileStateV1)(nil)

func newFile() *file {
	return &file{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newFileState() *fileStateV1 {
	return &fileStateV1{
		ID:        newTfString(),
		Src:       newTfString(),
		Dst:       newTfString(),
		Content:   newTfString(),
		Sum:       newTfString(),
		TmpDir:    newTfString(),
		Chmod:     newTfString(),
		Chown:     newTfString(),
		Transport: newEmbeddedTransport(),
	}
}

func (f *file) Name() string {
	return "enos_file"
}

func (f *file) Schema() *tfprotov5.Schema {
	return newFileState().Schema()
}

func (f *file) SetProviderConfig(providerConfig tftypes.Value) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.providerConfig.FromTerraform5Value(providerConfig)
}

func (f *file) GetProviderConfig() (*config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.providerConfig.Copy()
}

// ValidateResourceTypeConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (f *file) ValidateResourceTypeConfig(ctx context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	newState := newFileState()

	return transportUtil.ValidateResourceTypeConfig(ctx, newState, req)
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (f *file) UpgradeResourceState(ctx context.Context, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
	newState := newFileState()

	return transportUtil.UpgradeResourceState(ctx, newState, req)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (f *file) ReadResource(ctx context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	newState := newFileState()

	return transportUtil.ReadResource(ctx, newState, req)
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
	newState := newFileState()

	return transportUtil.ImportResourceState(ctx, newState, req)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (f *file) PlanResourceChange(ctx context.Context, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, error) {
	priorState := newFileState()
	proposedState := newFileState()

	res, transport, err := transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, f, req)
	if err != nil {
		return res, err
	}

	// ensure that computed attributes are unknown if we don't have a value
	if _, ok := proposedState.Sum.Get(); !ok {
		proposedState.Sum.Unknown = true
	}

	if _, ok := proposedState.ID.Get(); !ok {
		proposedState.ID.Unknown = true
	}

	// If we have unknown attributes we can't generate a valid Sum
	if !proposedState.hasUnknownAttributes() {
		// Load the file source
		src, srcType, err := proposedState.openSourceOrContent()
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
		defer src.Close() // nolint: staticcheck

		// Get the file's SHA256 sum, which we'll use to determine if the resource needs to be updated.
		sum, err := tfile.SHA256(src)
		if err != nil {
			err = wrapErrWithDiagnostics(err,
				"invalid configuration", "unable to obtain file SHA256 sum", srcType,
			)
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
		proposedState.Sum.Set(sum)
	}

	err = transportUtil.PlanMarshalPlannedState(ctx, res, proposedState, transport)

	return res, err
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (f *file) ApplyResourceChange(ctx context.Context, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
	priorState := newFileState()
	plannedState := newFileState()

	res, err := transportUtil.ApplyUnmarshalState(ctx, priorState, plannedState, req)
	if err != nil {
		return res, err
	}

	// If our prior state has an ID but our planned does not we're deleting.
	_, okprior := priorState.ID.Get()
	_, okplan := plannedState.ID.Get()
	if okprior && !okplan {
		res.NewState, err = marshalDelete(plannedState)
		return res, err
	}

	plannedState.ID.Set("static")

	transport, err := transportUtil.ApplyValidatePlannedAndBuildTransport(ctx, res, plannedState, f)
	if err != nil {
		return res, err
	}

	src, _, err := plannedState.openSourceOrContent()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}
	defer src.Close() //nolint: staticcheck

	// If we're missing a prior ID we haven't created it yet. If the prior and
	// planned sum don't match then we're updating.
	if !okprior || !priorState.Sum.Eq(plannedState.Sum) {
		ssh, err := transport.Client(ctx)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
		defer ssh.Close() //nolint: staticcheck

		// Get our copy args
		opts := []remoteflight.CopyFileRequestOpt{
			remoteflight.WithCopyFileContent(src),
			remoteflight.WithCopyFileDestination(plannedState.Dst.Value()),
		}
		if t, ok := plannedState.TmpDir.Get(); ok {
			opts = append(opts, remoteflight.WithCopyFileTmpDir(t))
		}
		if chmod, ok := plannedState.Chmod.Get(); ok {
			opts = append(opts, remoteflight.WithCopyFileChmod(chmod))
		}
		if chown, ok := plannedState.Chown.Get(); ok {
			opts = append(opts, remoteflight.WithCopyFileChmod(chown))
		}

		err = remoteflight.CopyFile(ctx, ssh, remoteflight.NewCopyFileRequest(opts...))
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
	}

	err = transportUtil.ApplyMarshalNewState(ctx, res, plannedState, transport)
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
					Name:     "sum",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "source",
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "destination",
					Type:     tftypes.String,
					Required: true,
				},
				{
					Name:      "content",
					Type:      tftypes.String,
					Optional:  true,
					Sensitive: true,
				},
				{
					Name:     "tmp_dir",
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "chmod",
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "chown",
					Type:     tftypes.String,
					Optional: true,
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

	src, okSrc := fs.Src.Get()
	cnt, okCnt := fs.Content.Get()

	if !okSrc && !okCnt {
		return newErrWithDiagnostics("invalid configuration", "you must provide either the source location or file content", "source")
	}

	if okSrc && okCnt {
		return newErrWithDiagnostics("invalid configuration", "you must provide only of of the source location or file content", "source")
	}

	if okSrc {
		f, err := tfile.Open(src)
		if err != nil {
			return newErrWithDiagnostics("invalid configuration", "unable to open source file", "source")
		}
		defer f.Close() // nolint: staticcheck
	}

	if okCnt && cnt == "" {
		return newErrWithDiagnostics("invalid configuration", "you must provide content", "content")
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (fs *fileStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id":          fs.ID,
		"source":      fs.Src,
		"destination": fs.Dst,
		"content":     fs.Content,
		"sum":         fs.Sum,
		"tmp_dir":     fs.TmpDir,
		"chmod":       fs.Chmod,
		"chown":       fs.Chown,
	})
	if err != nil {
		return err
	}

	if vals["transport"].IsKnown() {
		return fs.Transport.FromTerraform5Value(vals["transport"])
	}

	return nil
}

// Terraform5Type is the file state tftypes.Type.
func (fs *fileStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":          fs.ID.TFType(),
		"source":      fs.Src.TFType(),
		"destination": fs.Dst.TFType(),
		"content":     fs.Content.TFType(),
		"sum":         fs.Sum.TFType(),
		"tmp_dir":     fs.TmpDir.TFType(),
		"chmod":       fs.Chmod.TFType(),
		"chown":       fs.Chown.TFType(),
		"transport":   fs.Transport.Terraform5Type(),
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (fs *fileStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(fs.Terraform5Type(), map[string]tftypes.Value{
		"id":          fs.ID.TFValue(),
		"source":      fs.Src.TFValue(),
		"destination": fs.Dst.TFValue(),
		"content":     fs.Content.TFValue(),
		"sum":         fs.Sum.TFValue(),
		"tmp_dir":     fs.TmpDir.TFValue(),
		"chmod":       fs.Chmod.TFValue(),
		"chown":       fs.Chown.TFValue(),
		"transport":   fs.Transport.Terraform5Value(),
	})
}

// EmbeddedTransport is a pointer to the state's embedded transport
func (fs *fileStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return fs.Transport
}

// openSourceOrContent returns a stream of the source content
func (fs *fileStateV1) openSourceOrContent() (it.Copyable, string, error) {
	var err error
	var src it.Copyable
	var srcType string

	if srcVal, ok := fs.Src.Get(); ok {
		srcType = "source"
		src, err = tfile.Open(srcVal)
		if err != nil {
			err = wrapErrWithDiagnostics(err,
				"invalid configuration", "unable to open source file", srcType,
			)
			return src, srcType, err
		}
	} else if cntVal, ok := fs.Content.Get(); ok {
		srcType = "content"
		src = tfile.NewReader(cntVal)
	} else {
		return src, srcType, newErrWithDiagnostics("invalid configuration", "you must provide a source file or content", "source")
	}

	return src, srcType, nil
}

// hasUnknownAttributes determines if the source or content is not known
// yet.
func (fs *fileStateV1) hasUnknownAttributes() bool {
	return (fs.Src.Unknown || fs.Content.Unknown)
}
