// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hashicorp/enos-provider/internal/diags"
	"github.com/hashicorp/enos-provider/internal/remoteflight"
	resource "github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/enos-provider/internal/server/state"
	it "github.com/hashicorp/enos-provider/internal/transport"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type file struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resource.Resource = (*file)(nil)

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

	failureHandlers
}

var _ state.State = (*fileStateV1)(nil)

func newFile() *file {
	return &file{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newFileState() *fileStateV1 {
	transport := newEmbeddedTransport()
	fh := failureHandlers{TransportDebugFailureHandler(transport)}

	return &fileStateV1{
		ID:              newTfString(),
		Src:             newTfString(),
		Dst:             newTfString(),
		Content:         newTfString(),
		Sum:             newTfString(),
		TmpDir:          newTfString(),
		Chmod:           newTfString(),
		Chown:           newTfString(),
		Transport:       transport,
		failureHandlers: fh,
	}
}

func (f *file) Name() string {
	return "enos_file"
}

func (f *file) Schema() *tfprotov6.Schema {
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

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (f *file) ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	newState := newFileState()

	transportUtil.ValidateResourceConfig(ctx, newState, req, res)
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (f *file) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	newState := newFileState()

	transportUtil.UpgradeResourceState(ctx, newState, req, res)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (f *file) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	newState := newFileState()

	transportUtil.ReadResource(ctx, newState, req, res)
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
func (f *file) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	newState := newFileState()

	transportUtil.ImportResourceState(ctx, newState, req, res)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (f *file) PlanResourceChange(ctx context.Context, req resource.PlanResourceChangeRequest, res *resource.PlanResourceChangeResponse) {
	priorState := newFileState()
	proposedState := newFileState()
	res.PlannedState = proposedState

	transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, f, req, res)
	if diags.HasErrors(res.Diagnostics) {
		return
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
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Invalid Configuration", err))
			return
		}
		defer src.Close()

		// Get the file's SHA256 sum, which we'll use to determine if the resource needs to be updated.
		sum, err := tfile.SHA256(src)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
				"Invalid Configuration",
				fmt.Errorf("unable to obtain file SHA256 sum for %s, due to: %w", srcType, err),
			))

			return
		}
		proposedState.Sum.Set(sum)
	}
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (f *file) ApplyResourceChange(ctx context.Context, req resource.ApplyResourceChangeRequest, res *resource.ApplyResourceChangeResponse) {
	priorState := newFileState()
	plannedState := newFileState()
	res.NewState = plannedState

	transportUtil.ApplyUnmarshalState(ctx, priorState, plannedState, req, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	if req.IsDelete() {
		// nothing to do on delete
		return
	}

	plannedState.ID.Set("static")

	transport := transportUtil.ApplyValidatePlannedAndBuildTransport(ctx, plannedState, f, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	src, _, err := plannedState.openSourceOrContent()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Invalid Configuration", err))
		return
	}
	defer src.Close()

	_, okprior := priorState.ID.Get()
	// If we're missing a prior ID we haven't created it yet. If the prior and
	// planned sum don't match then we're updating.
	if !okprior || !priorState.Sum.Eq(plannedState.Sum) {
		client, err := transport.Client(ctx)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Invalid Configuration", err))
			return
		}
		defer client.Close()

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

		err = remoteflight.CopyFile(ctx, client, remoteflight.NewCopyFileRequest(opts...))
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Copy Error", err))
			return
		}
	}
}

// Schema is the file states Terraform schema.
func (fs *fileStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			DescriptionKind: tfprotov6.StringKindMarkdown,
			Description: docCaretToBacktick(`
The ^enos_file^ resource is capable of copying a local file to a remote destination
over an Enos transport.

When an SSH transport is used the resource is also capable of using the SSH agent. It will attempt
to connect to the agent socket as defined with the ^SSH_AUTH_SOCK^ environment variable.

^^^hcl
resource "enos_file" "foo" {
  source      = "/local/path/to/file.txt"
  destination = "/remote/destination/file.txt"
  content     = data.template_file.some_template.rendered

  transport = {
    ssh = {
      host             = "192.168.0.1"
      user             = "ubuntu"
      private_key_path = "/path/to/private/key.pem"
    }
  }
}
^^^
`),
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:        "id",
					Type:        tftypes.String,
					Computed:    true,
					Description: resourceStaticIDDescription,
				},
				{
					Name:        "sum",
					Type:        tftypes.String,
					Computed:    true,
					Description: "The SHA 256 sum of the source file. If the sum changes between runs the file will be uploaded again",
				},
				{
					Name:        "source",
					Type:        tftypes.String,
					Optional:    true,
					Description: "The file path to the source file to copy",
				},
				{
					Name:        "destination",
					Type:        tftypes.String,
					Required:    true,
					Description: "The file path on the remote host you wish to copy the file to",
				},
				{
					Name:        "content",
					Type:        tftypes.String,
					Optional:    true,
					Sensitive:   true,
					Description: "If the file does not exist locally you can provide the content as a string value and it will be written to the remote destination",
				},
				{
					Name:        "tmp_dir",
					Type:        tftypes.String,
					Description: "The location on disk to use for temporary files",
					Optional:    true,
				},
				{
					Name:        "chmod",
					Type:        tftypes.String,
					Description: "Configure the destination file mode",
					Optional:    true,
				},
				{
					Name:        "chown",
					Type:        tftypes.String,
					Description: "Configure the destination file owner",
					Optional:    true,
				},
				fs.Transport.SchemaAttributeTransport(supportsAll),
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
		return ValidationError("you must provide either the source location or file content")
	}

	if okSrc && okCnt {
		return ValidationError("you must provide only of the source location or file content")
	}

	if okSrc {
		f, err := tfile.Open(src)
		if err != nil {
			return ValidationError("unable to open source file", "source")
		}
		defer f.Close()
	}

	if okCnt && cnt == "" {
		return ValidationError("you must provide content", "content")
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

// Terraform5Value is the file state tftypes.Value.
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

// EmbeddedTransport is a pointer to the state's embedded transport.
func (fs *fileStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return fs.Transport
}

// openSourceOrContent returns a stream of the source content.
func (fs *fileStateV1) openSourceOrContent() (it.Copyable, string, error) {
	var err error
	var src it.Copyable
	var srcType string

	if srcVal, ok := fs.Src.Get(); ok {
		srcType = "source"
		src, err = tfile.Open(srcVal)
		if err != nil {
			return src, srcType, AttributePathError(
				err,
				fmt.Sprintf("unable to open source file: [%s]", srcType), "source",
			)
		}
	} else if cntVal, ok := fs.Content.Get(); ok {
		srcType = "content"
		src = tfile.NewReader(cntVal)
	} else {
		return src, srcType, errors.New("invalid configuration, you must provide a either a source file or content")
	}

	return src, srcType, nil
}

// hasUnknownAttributes determines if the source or content is not known
// yet.
func (fs *fileStateV1) hasUnknownAttributes() bool {
	return fs.Src.Unknown || fs.Content.Unknown
}
