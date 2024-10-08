// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/diags"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/random"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight"
	resource "github.com/hashicorp-forge/terraform-provider-enos/internal/server/resourcerouter"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/state"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	tfile "github.com/hashicorp-forge/terraform-provider-enos/internal/transport/file"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/ui"
)

type remoteExec struct {
	providerConfig *config
	mu             sync.Mutex

	stateFactory remoteExecStateFactory
}

var _ resource.Resource = (*remoteExec)(nil)

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

	failureHandlers
}

var _ state.State = (*remoteExecStateV1)(nil)

// remoteExecStateFactory a factory that can be used to override the default state creation, useful in tests.
type remoteExecStateFactory = func() *remoteExecStateV1

func newRemoteExec() *remoteExec {
	return &remoteExec{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
		stateFactory:   newRemoteExecStateV1,
	}
}

func newRemoteExecStateV1() *remoteExecStateV1 {
	transport := newEmbeddedTransport()
	fh := failureHandlers{
		TransportDebugFailureHandler(transport),
		GetApplicationLogsFailureHandler(transport, []string{}),
	}

	return &remoteExecStateV1{
		ID:              newTfString(),
		Env:             newTfStringMap(),
		Content:         newTfString(),
		Inline:          newTfStringSlice(),
		Scripts:         newTfStringSlice(),
		Sum:             newTfString(),
		Stderr:          newTfString(),
		Stdout:          newTfString(),
		Transport:       transport,
		failureHandlers: fh,
	}
}

func (r *remoteExec) Name() string {
	return "enos_remote_exec"
}

func (r *remoteExec) Schema() *tfprotov6.Schema {
	return r.stateFactory().Schema()
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
func (r *remoteExec) ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	newState := r.stateFactory()

	transportUtil.ValidateResourceConfig(ctx, newState, req, res)
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (r *remoteExec) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	newState := r.stateFactory()

	transportUtil.UpgradeResourceState(ctx, newState, req, res)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *remoteExec) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	newState := r.stateFactory()

	transportUtil.ReadResource(ctx, newState, req, res)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *remoteExec) PlanResourceChange(ctx context.Context, req resource.PlanResourceChangeRequest, res *resource.PlanResourceChangeResponse) {
	priorState := r.stateFactory()
	proposedState := r.stateFactory()
	res.PlannedState = proposedState

	transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, r, req, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	// Since content is optional we need to make sure we only update the sum
	// if we know it.
	if !proposedState.hasUnknownAttributes() {
		sha256, err := proposedState.config().computeSHA256(ctx)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
				"Invalid Configuration",
				fmt.Errorf("failed to read all scripts, due to: %w", err),
			))

			return
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
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *remoteExec) ApplyResourceChange(ctx context.Context, req resource.ApplyResourceChangeRequest, res *resource.ApplyResourceChangeResponse) {
	priorState := r.stateFactory()
	plannedState := r.stateFactory()
	res.NewState = plannedState

	transportUtil.ApplyUnmarshalState(ctx, priorState, plannedState, req, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	if req.IsDelete() {
		// nothing to do on delete
		return
	}

	transport := transportUtil.ApplyValidatePlannedAndBuildTransport(ctx, plannedState, r, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	// If our priorState Sum is blank then we're creating the resource. If
	// it's not blank and doesn't match the planned state we're updating.
	_, pok := priorState.ID.Get()
	if !pok {
		plannedState.ID.Set(random.ID())
	}

	priorSum, prsumok := priorState.Sum.Get()
	plannedSum, plsumok := plannedState.Sum.Get()

	if !pok || !prsumok || !plsumok || (priorSum != plannedSum) {
		client, err := transport.Client(ctx)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Transport Error", err))
			return
		}
		defer client.Close()

		ui, err := r.ExecuteCommands(ctx, plannedState, client)
		plannedState.Stdout.Set(ui.StdoutString())
		plannedState.Stderr.Set(ui.StderrString())
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
				"Execution Error",
				fmt.Errorf("failed to execute commands due to: %w%s", err, formatOutputIfExists(ui)),
			))

			return
		}

		// Make sure we set our planned sum if we didn't know it until apply time.
		if !plsumok {
			sha256, err := plannedState.config().computeSHA256(ctx)
			if err != nil {
				res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
					"Invalid Configuration",
					fmt.Errorf("failed to read all scripts, due to: %w", err),
				))

				return
			}
			plannedState.Sum.Set(sha256)
		}
	}
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
func (r *remoteExec) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	newState := r.stateFactory()

	transportUtil.ImportResourceState(ctx, newState, req, res)
}

// Schema is the file states Terraform schema.
func (s *remoteExecStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			DescriptionKind: tfprotov6.StringKindMarkdown,
			Description: docCaretToBacktick(`
The ^enos_remote_exec^ resource is capable of running scripts or commands on a remote instance over an SSH transport.

**Note**
Inline commands should not include double quotes, since the command will eventually be run as: ^sh -c "<your command>"^.
If a double quote must be included in the command it should be escaped as follows: ^\\\"^.
`),
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:        "id",
					Type:        tftypes.String,
					Computed:    true,
					Description: "A random ID number associated with the resource. This is created a single time during the initial 'apply' phase. It is utilized as a prefix when copying file contents to the remote target",
				},
				{
					Name:        "sum",
					Type:        tftypes.String,
					Computed:    true,
					Description: "A digest of the inline commands, source files, and environment variables. If the sum changes between runs all commands will execute again",
				},
				{
					Name: "environment",
					Type: tftypes.Map{
						ElementType: tftypes.String,
					},
					Optional:    true,
					Sensitive:   true,
					Description: "A map of key/value pairs to set as environment variable before running the commands or scripts. These values will be exported as environment variables when the commands are executed",
				},
				{
					Name: "inline",
					Type: tftypes.List{
						ElementType: tftypes.String,
					},
					Optional:    true,
					Description: "An array of commands to run",
				},
				{
					Name: "scripts",
					Type: tftypes.List{
						ElementType: tftypes.String,
					},
					Optional:    true,
					Description: "An array of paths to scripts to run",
				},
				{
					Name:        "content",
					Type:        tftypes.String,
					Optional:    true,
					Sensitive:   true,
					Description: "A string that represents a script body to execute",
				},
				{
					Name:        "stderr",
					Type:        tftypes.String,
					Computed:    true,
					Description: "The aggregate STDERR of all inline commnads, scripts, or content. If nothing is output this value will be set to a blank string",
				},
				{
					Name:        "stdout",
					Type:        tftypes.String,
					Computed:    true,
					Description: "The aggregate STDOUT of all inline commnads, scripts, or content. If nothing is output this value will be set to a blank string",
				},
				s.Transport.SchemaAttributeTransport(supportsSSH | supportsK8s | supportsNomad),
			},
		},
	}
}

// ExecuteCommands executes any commands or scripts and returns the STDOUT, STDERR,
// and any errors encountered.
func (r *remoteExec) ExecuteCommands(ctx context.Context, state *remoteExecStateV1, client it.Transport) (ui.UI, error) {
	var err error
	ui := ui.NewBuffered()

	if inline, ok := state.Inline.GetStrings(); ok {
		for _, cmd := range inline {
			select {
			case <-ctx.Done():
				return ui, fmt.Errorf("context deadline exceeded while running inline commands, due to: %w", ctx.Err())
			default:
			}

			// continue early if line has no commands
			if cmd == "" {
				continue
			}

			exec := func(cmd string) error {
				source := tfile.NewReader(cmd)
				defer source.Close()

				return r.copyAndRun(ctx, ui, client, source, "inline", state)
			}
			if err := exec(cmd); err != nil {
				return ui, fmt.Errorf("running inline command failed, due to: %w", err)
			}
		}
	}

	if scripts, ok := state.Scripts.GetStrings(); ok {
		for _, path := range scripts {
			exec := func(path string) error {
				script, err := tfile.Open(path)
				if err != nil {
					return fmt.Errorf("failed to open script file: [%s], due to: %w", path, err)
				}
				defer script.Close()

				return r.copyAndRun(ctx, ui, client, script, "script", state)
			}

			if err := exec(path); err != nil {
				return ui, fmt.Errorf("running script: [%s] failed, due to: %w", path, err)
			}
		}
	}

	if cont, ok := state.Content.Get(); ok {
		content := tfile.NewReader(cont)
		defer content.Close()

		err = r.copyAndRun(ctx, ui, client, content, "content", state)
		if err != nil {
			return ui, fmt.Errorf("running command content failed, due to: %w", err)
		}
	}

	return ui, nil
}

// copyAndRun copies the copyable source to the target using the configured transport,
// sets the environment variables and executes the content of the source.
// It returns STDOUT, STDERR, and any errors encountered.
func (r *remoteExec) copyAndRun(ctx context.Context, ui ui.UI, client it.Transport, src it.Copyable, srcType string, state *remoteExecStateV1) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	merr := &multierror.Error{}

	sha, err := tfile.SHA256(src)
	if err != nil {
		return fmt.Errorf("unable to determine %s SHA256 sum, due to: %w", srcType, err)
	}

	_, err = src.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("unable to seek to %s start, due to: %w", srcType, err)
	}

	// TODO: Eventually we'll probably have to support /tmp being mounted
	// with no exec. In those cases we'll have to make this configurable
	// or find another strategy for executing scripts.
	env, _ := state.Env.GetStrings()
	res, err := remoteflight.RunScript(ctx, client, remoteflight.NewRunScriptRequest(
		remoteflight.WithRunScriptContent(src),
		remoteflight.WithRunScriptDestination(fmt.Sprintf("/tmp/%s-%s.sh", state.ID.Value(), sha)),
		remoteflight.WithRunScriptEnv(env),
		remoteflight.WithRunScriptChmod("0777"),
	))
	merr = multierror.Append(merr, err)
	merr = multierror.Append(merr, ui.Append(res.Stdout, res.Stderr))

	return merr.ErrorOrNil()
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
		return ValidationError("you must provide one or more of content, inline commands or scripts")
	}

	// Make sure the scripts exist
	if scripts, ok := s.Scripts.GetStrings(); ok {
		var f it.Copyable
		var err error
		for _, path := range scripts {
			f, err = tfile.Open(path)
			if err != nil {
				return ValidationError(
					fmt.Sprintf("unable to open script file: [%s]", path),
					"scripts",
				)
			}
			defer f.Close()
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

// Terraform5Value is the file state tftypes.Value.
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

func (s *remoteExecStateV1) config() execConfig {
	return execConfig{
		Env:     s.Env,
		Content: s.Content,
		Inline:  s.Inline,
		Scripts: s.Scripts,
	}
}

func formatOutputIfExists(ui ui.UI) string {
	output := ui.CombinedOutput()
	if len(output) > 0 {
		return "\n\noutput:\n" + output
	}

	return ""
}
