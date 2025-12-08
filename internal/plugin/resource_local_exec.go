// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/diags"
	resource "github.com/hashicorp-forge/terraform-provider-enos/internal/server/resourcerouter"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/state"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	tfile "github.com/hashicorp-forge/terraform-provider-enos/internal/transport/file"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/ui"
)

type localExec struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resource.Resource = (*localExec)(nil)

type localExecStateV1 struct {
	ID         *tfString
	Env        *tfStringMap
	InheritEnv *tfBool
	Content    *tfString
	Inline     *tfStringSlice
	Scripts    *tfStringSlice
	Sum        *tfString
	Stderr     *tfString
	Stdout     *tfString

	failureHandlers
}

var _ state.State = (*localExecStateV1)(nil)

func newLocalExec() *localExec {
	return &localExec{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newLocalExecStateV1() *localExecStateV1 {
	return &localExecStateV1{
		ID:              newTfString(),
		Env:             newTfStringMap(),
		InheritEnv:      newTfBool(),
		Content:         newTfString(),
		Inline:          newTfStringSlice(),
		Scripts:         newTfStringSlice(),
		Sum:             newTfString(),
		Stderr:          newTfString(),
		Stdout:          newTfString(),
		failureHandlers: failureHandlers{},
	}
}

func (l *localExec) Name() string {
	return "enos_local_exec"
}

func (l *localExec) Schema() *tfprotov6.Schema {
	return newLocalExecStateV1().Schema()
}

func (l *localExec) SetProviderConfig(meta tftypes.Value) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.providerConfig.FromTerraform5Value(meta)
}

func (l *localExec) GetProviderConfig() (*config, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.providerConfig.Copy()
}

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (l *localExec) ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	newState := newLocalExecStateV1()

	transportUtil.ValidateResourceConfig(ctx, newState, req, res)
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (l *localExec) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	newState := newLocalExecStateV1()

	transportUtil.UpgradeResourceState(ctx, newState, req, res)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (l *localExec) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	newState := newLocalExecStateV1()

	transportUtil.ReadResource(ctx, newState, req, res)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (l *localExec) PlanResourceChange(ctx context.Context, req resource.PlanResourceChangeRequest, res *resource.PlanResourceChangeResponse) {
	priorState := newLocalExecStateV1()
	proposedState := newLocalExecStateV1()
	res.PlannedState = proposedState

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	err := priorState.FromTerraform5Value(req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	err = proposedState.FromTerraform5Value(req.ProposedNewState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	// Calculate the sum if we already know all of our attributes.
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
			// If we have Unknown attributes plan for a new sum and output.
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
func (l *localExec) ApplyResourceChange(ctx context.Context, req resource.ApplyResourceChangeRequest, res *resource.ApplyResourceChangeResponse) {
	priorState := newLocalExecStateV1()
	plannedState := newLocalExecStateV1()
	res.NewState = plannedState

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	err := plannedState.FromTerraform5Value(req.PlannedState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	err = priorState.FromTerraform5Value(req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		return
	}

	if req.IsDelete() {
		// nothing to do on delete
		return
	}
	plannedState.ID.Set("static")

	err = plannedState.Validate(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Validation Failure", err))
		return
	}

	// If our priorState Sum is blank then we're creating the resource. If
	// it's not blank and doesn't match the planned state we're updating.
	_, pok := priorState.ID.Get()
	priorSum, prsumok := priorState.Sum.Get()
	plannedSum, plsumok := plannedState.Sum.Get()

	if !pok || !prsumok || !plsumok || (priorSum != plannedSum) {
		ui, err := l.ExecuteCommands(ctx, plannedState)
		plannedState.Stdout.Set(ui.StdoutString())
		plannedState.Stderr.Set(ui.StderrString())
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic(
				"Execution Error",
				fmt.Errorf("failed to execute commands due to: %w%s", err, formatOutputIfExists(ui)),
			))

			return
		}
	}
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
func (l *localExec) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	newState := newLocalExecStateV1()

	transportUtil.ImportResourceState(ctx, newState, req, res)
}

// Schema is the file states Terraform schema.
func (s *localExecStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			DescriptionKind: tfprotov6.StringKindMarkdown,
			Description: docCaretToBacktick(`
The ^enos_local_exec^ resource is capable of running scripts or commands locally.
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
					Name:        "inherit_environment",
					Type:        tftypes.Bool,
					Optional:    true,
					Description: "Whether to inherit the all the environment variables of the current shell when running the local exec script",
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
			},
		},
	}
}

// ExecuteCommands executes any commands or scripts and returns the STDOUT, STDERR,
// and any errors encountered.
func (l *localExec) ExecuteCommands(ctx context.Context, state *localExecStateV1) (ui.UI, error) {
	ui := ui.NewBuffered()

	if inline, ok := state.Inline.GetStrings(); ok {
		for _, line := range inline {
			// continue early if line has no commands
			if line == "" {
				continue
			}

			source := strings.NewReader(line)

			err := l.copyAndRun(ctx, source, ui, state)
			if err != nil {
				return ui, fmt.Errorf("running inline command failed, due to: %w", err)
			}
		}
	}

	if scripts, ok := state.Scripts.GetStrings(); ok {
		for _, path := range scripts {
			source, err := os.Open(path)
			if err != nil {
				return ui, fmt.Errorf("failed to open script file: [%s], due to: %w", path, err)
			}
			defer source.Close()

			info, err := source.Stat()
			if err != nil {
				return ui, err
			}
			if info.IsDir() {
				return ui, fmt.Errorf("%s is a directory but should be a file", source.Name())
			}

			err = l.copyAndRun(ctx, source, ui, state)
			if err != nil {
				return ui, fmt.Errorf("running script: [%s] failed, due to: %w", path, err)
			}
		}
	}

	if cont, ok := state.Content.Get(); ok {
		source := strings.NewReader(cont)

		err := l.copyAndRun(ctx, source, ui, state)
		if err != nil {
			return ui, fmt.Errorf("running command content failed, due to: %w", err)
		}
	}

	return ui, nil
}

// copyAndRun takes an io.Reader and a pattern, creates an empty file (named according to pattern),
// copies the contents from the io.Reader to the empty file, makes that file executable,
// executes that file against bash, and then returns the output and any errors that were returned.
func (l *localExec) copyAndRun(ctx context.Context, source io.Reader, ui ui.UI, state *localExecStateV1) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default: // continues on because we haven't timed out
	}

	destination, err := os.CreateTemp("", "localExec-*")
	if err != nil {
		return err
	}
	defer os.Remove(destination.Name())

	_, err = io.Copy(destination, source)
	if err != nil {
		return err
	}

	if err := os.Chmod(destination.Name(), 0o755); err != nil {
		return fmt.Errorf(
			"failed to change ownership on script: %s, while executing commands, due to: %w",
			destination.Name(), err,
		)
	}

	//nolint:gosec// we know that we're executing user controlled code
	cmd := exec.CommandContext(ctx, "bash", destination.Name())
	cmd.Stdout = ui.Stdout()
	cmd.Stderr = ui.Stderr()

	if env, ok := state.Env.GetStrings(); ok {
		// env inheritance is on by default, hence the env should be inherited if the InheritEnv value is unknown
		if inherit, ok := state.InheritEnv.Get(); !ok || (ok && inherit) {
			cmd.Env = os.Environ()
		}
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute command due to: %w", err)
	}

	return nil
}

// Validate validates the configuration. This will validate the source file
// exists and that the transport configuration is valid.
func (s *localExecStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Make sure that we have content, inline commands or scripts
	_, okc := s.Content.Get()
	_, oki := s.Inline.Get()
	scripts, oks := s.Scripts.GetStrings()
	if !okc && !oki && !oks {
		return ValidationError("you must provide one of content, inline commands, or scripts")
	}

	// Make sure the scripts exist
	var f it.Copyable
	var err error
	for _, path := range scripts {
		f, err = tfile.Open(path)
		if err != nil {
			return AttributePathError(fmt.Errorf("validation error, unable to open script file: [%s], due to: %w", path, err),
				"scripts",
			)
		}
		defer f.Close()
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *localExecStateV1) FromTerraform5Value(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]any{
		"id":                  s.ID,
		"content":             s.Content,
		"sum":                 s.Sum,
		"stdout":              s.Stdout,
		"stderr":              s.Stderr,
		"environment":         s.Env,
		"inherit_environment": s.InheritEnv,
		"inline":              s.Inline,
		"scripts":             s.Scripts,
	})
	if err != nil {
		return err
	}

	return nil
}

// Terraform5Type is the file state tftypes.Type.
func (s *localExecStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":                  s.ID.TFType(),
		"sum":                 s.Sum.TFType(),
		"stdout":              s.Stdout.TFType(),
		"stderr":              s.Stderr.TFType(),
		"environment":         s.Env.TFType(),
		"inherit_environment": s.InheritEnv.TFType(),
		"inline":              s.Inline.TFType(),
		"scripts":             s.Scripts.TFType(),
		"content":             s.Content.TFType(),
	}}
}

// Terraform5Value is the file state tftypes.Value.
func (s *localExecStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":                  s.ID.TFValue(),
		"sum":                 s.Sum.TFValue(),
		"stdout":              s.Stdout.TFValue(),
		"stderr":              s.Stderr.TFValue(),
		"content":             s.Content.TFValue(),
		"inline":              s.Inline.TFValue(),
		"scripts":             s.Scripts.TFValue(),
		"environment":         s.Env.TFValue(),
		"inherit_environment": s.InheritEnv.TFValue(),
	})
}

func (s *localExecStateV1) hasUnknownAttributes() bool {
	if s.Content.Unknown || s.Scripts.Unknown || s.Inline.Unknown || s.Env.Unknown {
		return true
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

	if _, ok := s.Scripts.Get(); ok {
		if !s.Scripts.FullyKnown() {
			return true
		}
	}

	return false
}

func (s *localExecStateV1) config() execConfig {
	return execConfig{
		Env:     s.Env,
		Content: s.Content,
		Inline:  s.Inline,
		Scripts: s.Scripts,
	}
}
