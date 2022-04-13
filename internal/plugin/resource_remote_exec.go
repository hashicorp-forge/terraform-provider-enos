package plugin

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
	"github.com/hashicorp/enos-provider/internal/ui"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type remoteExec struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resourcerouter.Resource = (*remoteExec)(nil)

type remoteExecStateV1 struct {
	ID        *tfString
	Env       *tfStringMap
	Content   *tfString
	Inline    *tfStringSlice
	Scripts   *tfStringSlice
	Sum       *tfString
	Stderr    *tfString
	Stdout    *tfString
	Transport *embeddedTransportV1
}

var _ State = (*remoteExecStateV1)(nil)

func newRemoteExec() *remoteExec {
	return &remoteExec{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newRemoteExecStateV1() *remoteExecStateV1 {
	return &remoteExecStateV1{
		ID:        newTfString(),
		Env:       newTfStringMap(),
		Content:   newTfString(),
		Inline:    newTfStringSlice(),
		Scripts:   newTfStringSlice(),
		Sum:       newTfString(),
		Stderr:    newTfString(),
		Stdout:    newTfString(),
		Transport: newEmbeddedTransport(),
	}
}

func (r *remoteExec) Name() string {
	return "enos_remote_exec"
}

func (r *remoteExec) Schema() *tfprotov6.Schema {
	return newRemoteExecStateV1().Schema()
}

func (r *remoteExec) SetProviderConfig(meta tftypes.Value) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.FromTerraform5Value(meta)
}

func (r *remoteExec) GetProviderConfig() (*config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.Copy()
}

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (r *remoteExec) ValidateResourceConfig(ctx context.Context, req *tfprotov6.ValidateResourceConfigRequest) (*tfprotov6.ValidateResourceConfigResponse, error) {
	newState := newRemoteExecStateV1()

	return transportUtil.ValidateResourceConfig(ctx, newState, req)
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (r *remoteExec) UpgradeResourceState(ctx context.Context, req *tfprotov6.UpgradeResourceStateRequest) (*tfprotov6.UpgradeResourceStateResponse, error) {
	newState := newRemoteExecStateV1()

	return transportUtil.UpgradeResourceState(ctx, newState, req)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *remoteExec) ReadResource(ctx context.Context, req *tfprotov6.ReadResourceRequest) (*tfprotov6.ReadResourceResponse, error) {
	newState := newRemoteExecStateV1()

	return transportUtil.ReadResource(ctx, newState, req)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *remoteExec) PlanResourceChange(ctx context.Context, req *tfprotov6.PlanResourceChangeRequest) (*tfprotov6.PlanResourceChangeResponse, error) {
	priorState := newRemoteExecStateV1()
	proposedState := newRemoteExecStateV1()

	res, transport, err := transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, r, req)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	// Since content is optional we need to make sure we only update the sum
	// if we known it.
	if !proposedState.hasUnknownAttributes() {
		sha256, err := r.SHA256(ctx, proposedState)
		if err != nil {
			err = wrapErrWithDiagnostics(err, "invalid configuration", "unable to read all scripts", "scripts")
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
		proposedState.Sum.Set(sha256)
	} else if _, ok := proposedState.Sum.Get(); !ok {
		proposedState.Sum.Unknown = true
	}

	// If our prior ID is blank we're creating the resource.
	if _, ok := priorState.ID.Get(); !ok {
		proposedState.ID.Unknown = true
		// When we create we need to ensure that we plan unknown output.
		proposedState.Stdout.Unknown = true
		proposedState.Stderr.Unknown = true
	} else {
		// We have a prior ID so we're either updating or staying the same.
		if proposedState.hasUnknownAttributes() {
			// If we have unknown attributes plan for a new sum and output.
			proposedState.Sum.Unknown = true
			proposedState.Stdout.Unknown = true
			proposedState.Stderr.Unknown = true
		} else if priorSum, ok := priorState.Sum.Get(); ok {
			if proposedSum, ok := proposedState.Sum.Get(); ok {
				if priorSum != proposedSum {
					// If we have a new sum and it doesn't match the old one, we're
					// updating and need to plan for new output.
					proposedState.Stdout.Unknown = true
					proposedState.Stderr.Unknown = true
				}
			}
		}
	}

	err = transportUtil.PlanMarshalPlannedState(ctx, res, proposedState, transport)
	if err != nil {
		return res, err
	}

	return res, err
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *remoteExec) ApplyResourceChange(ctx context.Context, req *tfprotov6.ApplyResourceChangeRequest) (*tfprotov6.ApplyResourceChangeResponse, error) {
	priorState := newRemoteExecStateV1()
	plannedState := newRemoteExecStateV1()

	res, err := transportUtil.ApplyUnmarshalState(ctx, priorState, plannedState, req)
	if err != nil {
		return res, err
	}

	if plannedState.shouldDelete() {
		// Delete the resource
		res.NewState, err = marshalDelete(plannedState)
		return res, err
	}
	plannedState.ID.Set("static")

	transport, err := transportUtil.ApplyValidatePlannedAndBuildTransport(ctx, res, plannedState, r)
	if err != nil {
		return res, err
	}

	// If our priorState Sum is blank then we're creating the resource. If
	// it's not blank and doesn't match the planned state we're updating.
	_, pok := priorState.ID.Get()
	priorSum, prsumok := priorState.Sum.Get()
	plannedSum, plsumok := plannedState.Sum.Get()

	if !pok || !prsumok || !plsumok || (priorSum != plannedSum) {
		ssh, err := transport.Client(ctx)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
		defer ssh.Close() //nolint: staticcheck

		ui, err := r.ExecuteCommands(ctx, plannedState, ssh)
		plannedState.Stdout.Set(ui.Stdout().String())
		plannedState.Stderr.Set(ui.Stderr().String())
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
	}

	err = transportUtil.ApplyMarshalNewState(ctx, res, plannedState, transport)

	return res, err
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
func (r *remoteExec) ImportResourceState(ctx context.Context, req *tfprotov6.ImportResourceStateRequest) (*tfprotov6.ImportResourceStateResponse, error) {
	newState := newRemoteExecStateV1()

	return transportUtil.ImportResourceState(ctx, newState, req)
}

// Schema is the file states Terraform schema.
func (s *remoteExecStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			Attributes: []*tfprotov6.SchemaAttribute{
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
					Name: "environment",
					Type: tftypes.Map{
						ElementType: tftypes.String,
					},
					Optional:  true,
					Sensitive: true,
				},
				{
					Name: "inline",
					Type: tftypes.List{
						ElementType: tftypes.String,
					},
					Optional: true,
				},
				{
					Name: "scripts",
					Type: tftypes.List{
						ElementType: tftypes.String,
					},
					Optional: true,
				},
				{
					Name:      "content",
					Type:      tftypes.String,
					Optional:  true,
					Sensitive: true,
				},
				{
					Name:     "stderr",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "stdout",
					Type:     tftypes.String,
					Computed: true,
				},
				s.Transport.SchemaAttributeTransport(),
			},
		},
	}
}

// SHA256 is the aggregate sum of the resource
func (r *remoteExec) SHA256(ctx context.Context, state *remoteExecStateV1) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// We're probably overthinking this but this is a sha256 sum of the
	// aggregate of the inline commands, the rendered content, and scripts.
	ag := strings.Builder{}

	if cont, ok := state.Content.Get(); ok {
		content := tfile.NewReader(cont)
		defer content.Close()

		sha, err := tfile.SHA256(content)
		if err != nil {
			return "", wrapErrWithDiagnostics(
				err, "invalid configuration", "unable to determine content SHA256 sum", "content",
			)
		}

		ag.WriteString(sha)
	}

	if inline, ok := state.Inline.GetStrings(); ok {
		for _, cmd := range inline {
			ag.WriteString(command.SHA256(command.New(cmd)))
		}
	}

	if scripts, ok := state.Scripts.GetStrings(); ok {
		var sha string
		var file it.Copyable
		var err error
		for _, path := range scripts {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
			}

			file, err = tfile.Open(path)
			if err != nil {
				return "", wrapErrWithDiagnostics(
					err, "invalid configuration", "unable to open script file", "scripts",
				)
			}
			defer file.Close() // nolint: staticcheck

			sha, err = tfile.SHA256(file)
			if err != nil {
				return "", wrapErrWithDiagnostics(
					err, "invalid configuration", "unable to determine script file SHA256 sum", "scripts",
				)
			}

			ag.WriteString(sha)
		}
	}

	return fmt.Sprintf("%x", sha256.Sum256([]byte(ag.String()))), nil
}

// ExecuteCommands executes any commands or scripts and returns the STDOUT, STDERR,
// and any errors encountered.
func (r *remoteExec) ExecuteCommands(ctx context.Context, state *remoteExecStateV1, ssh it.Transport) (ui.UI, error) {
	var err error
	merr := &multierror.Error{}
	ui := ui.NewBuffered()
	env, _ := state.Env.GetStrings()

	if inline, ok := state.Inline.GetStrings(); ok {
		for _, cmd := range inline {
			select {
			case <-ctx.Done():
				return ui, wrapErrWithDiagnostics(
					ctx.Err(), "timed out", "context deadline exceeded while running inline commands",
				)
			default:
			}

			stdout, stderr, err := ssh.Run(ctx, command.New(cmd, command.WithEnvVars(env)))
			merr = multierror.Append(merr, err)
			merr = multierror.Append(merr, ui.Append(stdout, stderr))
			if err := merr.ErrorOrNil(); err != nil {
				return ui, wrapErrWithDiagnostics(
					err, "command failed", fmt.Sprintf("running inline command failed: %s", err.Error()),
				)
			}
		}
	}

	if scripts, ok := state.Scripts.GetStrings(); ok {
		for _, path := range scripts {
			script, err := tfile.Open(path)
			if err != nil {
				return ui, wrapErrWithDiagnostics(
					err, "invalid configuration", "unable to open script file", "scripts",
				)
			}
			defer script.Close() // nolint: staticcheck

			err = r.copyAndRun(ctx, ui, ssh, script, "script", env)
			if err != nil {
				return ui, err
			}
		}
	}

	if cont, ok := state.Content.Get(); ok {
		content := tfile.NewReader(cont)
		defer content.Close()

		err = r.copyAndRun(ctx, ui, ssh, content, "content", env)
		if err != nil {
			return ui, err
		}
	}

	return ui, nil
}

// copyAndRun copies the copyable source to the target using the SSH transport,
// sets the environment variables and executes the content of the source.
// It returns STDOUT, STDERR, and any errors encountered.
func (r *remoteExec) copyAndRun(ctx context.Context, ui ui.UI, ssh it.Transport, src it.Copyable, srcType string, env map[string]string) error {
	select {
	case <-ctx.Done():
		return wrapErrWithDiagnostics(
			ctx.Err(), "timed out", "context deadline exceeded while running scripts",
		)
	default:
	}

	merr := &multierror.Error{}

	sha, err := tfile.SHA256(src)
	if err != nil {
		return wrapErrWithDiagnostics(
			err, "invalid configuration", fmt.Sprintf("unable to determine %s SHA256 sum", srcType), srcType,
		)
	}

	_, err = src.Seek(0, io.SeekStart)
	if err != nil {
		return wrapErrWithDiagnostics(
			err, "invalid configuration", fmt.Sprintf("unable to seek to %s start", srcType), srcType,
		)
	}

	// TODO: Eventually we'll probably have to support /tmp being mounted
	// with no exec. In those cases we'll have to make this configurable
	// or find another strategy for executing scripts.
	res, err := remoteflight.RunScript(ctx, ssh, remoteflight.NewRunScriptRequest(
		remoteflight.WithRunScriptContent(src),
		remoteflight.WithRunScriptDestination(fmt.Sprintf("/tmp/%s.sh", sha)),
		remoteflight.WithRunScriptEnv(env),
		remoteflight.WithRunScriptChmod("0777"),
	))
	merr = multierror.Append(merr, err)
	merr = multierror.Append(merr, ui.Append(res.Stdout, res.Stderr))

	if merr.ErrorOrNil() != nil {
		return wrapErrWithDiagnostics(merr.ErrorOrNil(), "command failed", merr.Error())
	}

	return nil
}

// Validate validates the configuration. This will validate the source file
// exists and that the transport configuration is valid.
func (s *remoteExecStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Make sure that we have content, inline commands or scripts
	_, okcnt := s.Content.Get()
	_, okin := s.Inline.Get()
	_, okscr := s.Scripts.Get()

	if !okcnt && !okin && !okscr {
		return newErrWithDiagnostics("invalid configuration", "you must provide content, inline commands or scripts", "content")
	}

	// Make sure the scripts exist
	if scripts, ok := s.Scripts.GetStrings(); ok {
		var f it.Copyable
		var err error
		for _, path := range scripts {
			f, err = tfile.Open(path)
			if err != nil {
				return wrapErrWithDiagnostics(
					err, "invalid configuration", "unable to open script file", "scripts",
				)
			}
			defer f.Close() // nolint: staticcheck
		}
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *remoteExecStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id":          s.ID,
		"content":     s.Content,
		"sum":         s.Sum,
		"stdout":      s.Stdout,
		"stderr":      s.Stderr,
		"environment": s.Env,
		"inline":      s.Inline,
		"scripts":     s.Scripts,
	})
	if err != nil {
		return err
	}

	if vals["transport"].IsKnown() {
		return s.Transport.FromTerraform5Value(vals["transport"])
	}

	return nil
}

// Terraform5Type is the file state tftypes.Type.
func (s *remoteExecStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":          s.ID.TFType(),
		"sum":         s.Sum.TFType(),
		"stdout":      s.Stdout.TFType(),
		"stderr":      s.Stderr.TFType(),
		"environment": s.Env.TFType(),
		"inline":      s.Inline.TFType(),
		"scripts":     s.Scripts.TFType(),
		"content":     s.Content.TFType(),
		"transport":   s.Transport.Terraform5Type(),
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (s *remoteExecStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":          s.ID.TFValue(),
		"sum":         s.Sum.TFValue(),
		"stdout":      s.Stdout.TFValue(),
		"stderr":      s.Stderr.TFValue(),
		"content":     s.Content.TFValue(),
		"transport":   s.Transport.Terraform5Value(),
		"inline":      s.Inline.TFValue(),
		"scripts":     s.Scripts.TFValue(),
		"environment": s.Env.TFValue(),
	})
}

func (s *remoteExecStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}

func (s *remoteExecStateV1) shouldDelete() bool {
	_, okcnt := s.Content.Get()
	_, okin := s.Inline.Get()
	_, okscr := s.Scripts.Get()

	if !okcnt && !okin && !okscr {
		return true
	}

	return false
}

func (s *remoteExecStateV1) hasUnknownAttributes() bool {
	if s.Content.Unknown || s.Scripts.Unknown || s.Inline.Unknown || s.Env.Unknown {
		return true
	}

	if _, ok := s.Scripts.Get(); ok {
		if !s.Scripts.FullyKnown() {
			return true
		}
	}

	if _, ok := s.Inline.Get(); ok {
		if !s.Inline.FullyKnown() {
			return true
		}
	}

	if _, ok := s.Env.Get(); ok {
		if !s.Env.FullyKnown() {
			return true
		}
	}

	return false
}
