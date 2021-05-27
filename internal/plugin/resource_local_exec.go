package plugin

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
	"github.com/hashicorp/enos-provider/internal/ui"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tftypes"
)

type localExec struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resourcerouter.Resource = (*localExec)(nil)

type localExecStateV1 struct {
	ID      string
	Env     map[string]string
	Content string
	Inline  []string
	Scripts []string
	Sum     string
	Stderr  string
	Stdout  string
}

var _ State = (*localExecStateV1)(nil)

func newLocalExec() *localExec {
	return &localExec{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newLocalExecStateV1() *localExecStateV1 {
	return &localExecStateV1{}
}

func (l *localExec) Name() string {
	return "enos_local_exec"
}

func (l *localExec) Schema() *tfprotov5.Schema {
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

// ValidateResourceTypeConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (l *localExec) ValidateResourceTypeConfig(ctx context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	newState := newLocalExecStateV1()

	res := &tfprotov5.ValidateResourceTypeConfigResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	err := unmarshal(newState, req.Config)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	return res, err
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (l *localExec) UpgradeResourceState(ctx context.Context, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
	newState := newLocalExecStateV1()

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
func (l *localExec) ReadResource(ctx context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	newState := newLocalExecStateV1()

	res := &tfprotov5.ReadResourceResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

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
func (l *localExec) PlanResourceChange(ctx context.Context, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, error) {
	priorState := newLocalExecStateV1()
	proposedState := newLocalExecStateV1()

	res := &tfprotov5.PlanResourceChangeResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	err := unmarshal(priorState, req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	err = unmarshal(proposedState, req.ProposedNewState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	// Calculate the sum if we already know all of our attributes.
	if !proposedState.hasUnknownAttributes() {
		sha256, err := l.SHA256(ctx, proposedState)
		if err != nil {
			err = wrapErrWithDiagnostics(err, "invalid configuration", "unable to read all scripts", "scripts")
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
		proposedState.Sum = sha256
	}

	// If our prior ID is blank we're creating the resource.
	if priorState.ID == "" {
		// When we create we need to ensure that we plan unknown output.
		proposedState.Stdout = UnknownString
		proposedState.Stderr = UnknownString
	} else {
		// We have a prior ID so we're either updating or staying the same.
		if proposedState.hasUnknownAttributes() {
			// If we have unknown attributes plan for a new sum and output.
			proposedState.Sum = UnknownString
			proposedState.Stdout = UnknownString
			proposedState.Stderr = UnknownString
		} else if priorState.Sum != "" && priorState.Sum != proposedState.Sum {
			// If we have a new sum and it doesn't match the old one, we're
			// updating and need to plan for new output.
			proposedState.Stdout = UnknownString
			proposedState.Stderr = UnknownString
		}
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
func (l *localExec) ApplyResourceChange(ctx context.Context, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
	priorState := newLocalExecStateV1()
	plannedState := newLocalExecStateV1()

	res := &tfprotov5.ApplyResourceChangeResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	err := unmarshal(plannedState, req.PlannedState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	err = unmarshal(priorState, req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	if plannedState.shouldDelete() {
		// Delete the resource
		res.NewState, err = marshalDelete(plannedState)
		return res, err
	}
	plannedState.ID = "static"

	err = plannedState.Validate(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	// If our priorState Sum is blank then we're creating the resource. If
	// it's not blank and doesn't match the planned state we're updating.
	if priorState.ID == "" || (priorState.Sum != "" && priorState.Sum != plannedState.Sum) {
		ui, err := l.ExecuteCommands(ctx, plannedState)
		plannedState.Stdout = ui.Stdout().String()
		plannedState.Stderr = ui.Stderr().String()
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
	}

	res.NewState, err = marshal(plannedState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	return res, err
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
func (l *localExec) ImportResourceState(ctx context.Context, req *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
	newState := newLocalExecStateV1()

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

	importState, err := marshal(newState)
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

// Schema is the file states Terraform schema.
func (s *localExecStateV1) Schema() *tfprotov5.Schema {
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
					Name: "environment",
					Type: tftypes.Map{
						AttributeType: tftypes.String,
					},
					Optional: true,
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
			},
		},
	}
}

// SHA256 is the aggregate sum of the resource
func (l *localExec) SHA256(ctx context.Context, state *localExecStateV1) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// We're probably overthinking this but this is a sha256 sum of the
	// aggregate of the environment variables, inline commands, and scripts.
	ag := strings.Builder{}

	if state.Content != "" {
		content := tfile.NewReader(state.Content)
		defer content.Close()

		sha, err := tfile.SHA256(content)
		if err != nil {
			return "", wrapErrWithDiagnostics(
				err, "invalid configuration", "unable to determine content SHA256 sum", "content",
			)
		}

		ag.WriteString(sha)
	}

	for _, cmd := range state.Inline {
		ag.WriteString(command.SHA256(command.New(cmd, command.WithEnvVars(state.Env))))
	}

	var sha string
	var file it.Copyable
	var err error
	for _, path := range state.Scripts {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		file, err = tfile.Open(path)
		defer file.Close() // nolint: staticcheck
		if err != nil {
			return "", wrapErrWithDiagnostics(
				err, "invalid configuration", "unable to open script file", "scripts",
			)
		}

		sha, err = tfile.SHA256(file)
		if err != nil {
			return "", wrapErrWithDiagnostics(
				err, "invalid configuration", "unable to determine script file SHA256 sum", "scripts",
			)
		}

		ag.WriteString(sha)
	}

	return fmt.Sprintf("%x", sha256.Sum256([]byte(ag.String()))), nil
}

// ExecuteCommands executes any commands or scripts and returns the STDOUT, STDERR,
// and any errors encountered.
func (l *localExec) ExecuteCommands(ctx context.Context, state *localExecStateV1) (ui.UI, error) {
	ui := ui.NewBuffered()

	for _, line := range state.Inline {
		// continue early if line has no commands
		if line == "" {
			continue
		}

		source := strings.NewReader(line)

		err := l.copyAndRun(ctx, source, ui, state)
		if err != nil {
			return ui, err
		}
	}

	for _, path := range state.Scripts {
		source, err := os.Open(path)
		if err != nil {
			return ui, err
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
			return ui, err
		}
	}

	if state.Content != "" {
		source := strings.NewReader(state.Content)

		err := l.copyAndRun(ctx, source, ui, state)
		if err != nil {
			return ui, err
		}
	}

	return ui, nil
}

// copyAndRun takes an io.Reader and a pattern, creates an empty file (named according to pattern),
// copies the contents from the io.Reader to the empty file, makes that file executable,
// executes that file against bash, and then returns the output and any errors that were returned.
func (l *localExec) copyAndRun(ctx context.Context, source io.Reader, ui ui.UI, state *localExecStateV1) (err error) {
	select {
	case <-ctx.Done():
		err = wrapErrWithDiagnostics(ctx.Err(), "timed out", "while executing commands")
		return err
	default: // continues on because we haven't timed out
	}

	merr := &multierror.Error{}

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
		return wrapErrWithDiagnostics(
			err, "command failed", fmt.Sprintf("while changing ownership on script: %s", destination.Name()),
		)
	}

	cmd := exec.CommandContext(ctx, "bash", destination.Name())

	for k, v := range state.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdoutP, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderrP, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	// we'll collect errors that can occur throughout the execution of the command
	// and return them all at once if we found an issue
	merr = multierror.Append(merr, cmd.Start())

	stdoutBuffer, err := io.ReadAll(stdoutP)
	merr = multierror.Append(merr, err, ui.Append(string(stdoutBuffer), ""))

	stderrBuffer, err := io.ReadAll(stderrP)
	merr = multierror.Append(merr, err, ui.Append("", string(stderrBuffer)))

	merr = multierror.Append(merr, cmd.Wait())

	if merr.ErrorOrNil() != nil {
		return wrapErrWithDiagnostics(
			merr.ErrorOrNil(), "command failed", fmt.Sprintf("executing script: %s", merr.Error()),
		)
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
	if s.Content == "" && len(s.Inline) == 0 && len(s.Scripts) == 0 {
		return newErrWithDiagnostics("invalid configuration", "you must provide content, inline commands or scripts", "content")
	}

	// Make sure the scripts exist
	var f it.Copyable
	var err error
	for _, path := range s.Scripts {
		f, err = tfile.Open(path)
		defer f.Close() // nolint: staticcheck

		if err != nil {
			return wrapErrWithDiagnostics(
				err, "invalid configuration", "unable to open script file", "scripts",
			)
		}
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *localExecStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id":      &s.ID,
		"content": &s.Content,
		"sum":     &s.Sum,
		"stdout":  &s.Stdout,
		"stderr":  &s.Stderr,
	})
	if err != nil {
		return err
	}

	env, ok := vals["environment"]
	if ok {
		s.Env, err = tfUnmarshalStringMap(env)
		if err != nil {
			return err
		}
	}

	inline, ok := vals["inline"]
	if ok {
		s.Inline, err = tfUnmarshalStringSlice(inline)
		if err != nil {
			return err
		}
	}

	scripts, ok := vals["scripts"]
	if ok {
		s.Scripts, err = tfUnmarshalStringSlice(scripts)
		if err != nil {
			return err
		}
	}

	return nil
}

// Terraform5Type is the file state tftypes.Type.
func (s *localExecStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":          tftypes.String,
		"sum":         tftypes.String,
		"stdout":      tftypes.String,
		"stderr":      tftypes.String,
		"environment": tftypes.Map{AttributeType: tftypes.String},
		"inline":      tftypes.List{ElementType: tftypes.String},
		"scripts":     tftypes.List{ElementType: tftypes.String},
		"content":     tftypes.String,
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (s *localExecStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":          tfMarshalStringValue(s.ID),
		"sum":         tfMarshalStringValue(s.Sum),
		"stdout":      tfMarshalStringAllowBlank(s.Stdout),
		"stderr":      tfMarshalStringAllowBlank(s.Stderr),
		"content":     tfMarshalStringOptionalValue(s.Content),
		"inline":      tfMarshalStringSlice(s.Inline),
		"scripts":     tfMarshalStringSlice(s.Scripts),
		"environment": tfMarshalStringMap(s.Env),
	})
}

func (s *localExecStateV1) shouldDelete() bool {
	if s.Content == "" && len(s.Inline) == 0 && len(s.Scripts) == 0 {
		return true
	}

	return false
}

func (s *localExecStateV1) hasUnknownAttributes() bool {
	if s.Content == UnknownString {
		return true
	}

	for _, ary := range [][]string{s.Scripts, s.Inline} {
		for _, val := range ary {
			if val == UnknownString {
				return true
			}
		}
	}

	for _, val := range s.Env {
		if val == UnknownString {
			return true
		}
	}

	return false
}
