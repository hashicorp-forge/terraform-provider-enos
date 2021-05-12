package plugin

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
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

type remoteExec struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resourcerouter.Resource = (*remoteExec)(nil)

type remoteExecStateV1 struct {
	ID        string
	Env       map[string]string
	Content   string
	Inline    []string
	Scripts   []string
	Sum       string
	Stderr    string
	Stdout    string
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
		Env:       map[string]string{},
		Inline:    []string{},
		Scripts:   []string{},
		Transport: newEmbeddedTransport(),
	}
}

func (r *remoteExec) Name() string {
	return "enos_remote_exec"
}

func (r *remoteExec) Schema() *tfprotov5.Schema {
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

// ValidateResourceTypeConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (r *remoteExec) ValidateResourceTypeConfig(ctx context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	newState := newRemoteExecStateV1()

	return transportUtil.ValidateResourceTypeConfig(ctx, newState, req)
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (r *remoteExec) UpgradeResourceState(ctx context.Context, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
	newState := newRemoteExecStateV1()

	return transportUtil.UpgradeResourceState(ctx, newState, req)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *remoteExec) ReadResource(ctx context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	newState := newRemoteExecStateV1()

	return transportUtil.ReadResource(ctx, newState, req)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *remoteExec) PlanResourceChange(ctx context.Context, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, error) {
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
		proposedState.Sum = sha256
	}

	err = transportUtil.PlanMarshalPlannedState(ctx, res, proposedState, transport)
	if err != nil {
		return res, err
	}

	return res, err
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *remoteExec) ApplyResourceChange(ctx context.Context, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
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
	plannedState.ID = "static"

	transport, err := transportUtil.ApplyValidatePlannedAndBuildTransport(ctx, res, plannedState, r)
	if err != nil {
		return res, err
	}

	// If our priorState Sum is blank then we're creating the resource. If
	// it's not blank and doesn't match the planned state we're updating.
	if priorState.ID == "" || (priorState.Sum != "" && priorState.Sum != plannedState.Sum) {
		ssh, err := transport.Client(ctx)
		defer ssh.Close() //nolint: staticcheck
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}

		ui, err := r.ExecuteCommands(ctx, plannedState, ssh)
		plannedState.Stdout = ui.Stdout().String()
		plannedState.Stderr = ui.Stderr().String()
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
	}

	if plannedState.Stderr == "" {
		plannedState.Stderr = NullComputedString
	}

	if plannedState.Stdout == "" {
		plannedState.Stdout = NullComputedString
	}

	err = transportUtil.ApplyMarshalNewState(ctx, res, plannedState, transport)

	return res, err
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
func (r *remoteExec) ImportResourceState(ctx context.Context, req *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
	newState := newRemoteExecStateV1()

	return transportUtil.ImportResourceState(ctx, newState, req)
}

// Schema is the file states Terraform schema.
func (s *remoteExecStateV1) Schema() *tfprotov5.Schema {
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
func (r *remoteExec) ExecuteCommands(ctx context.Context, state *remoteExecStateV1, ssh it.Transport) (ui.UI, error) {
	var err error
	merr := &multierror.Error{}
	ui := ui.NewBuffered()

	for _, cmd := range state.Inline {
		select {
		case <-ctx.Done():
			return ui, wrapErrWithDiagnostics(
				ctx.Err(), "timed out", "context deadline exceeded while running inline commands",
			)
		default:
		}

		stdout, stderr, err := ssh.Run(ctx, command.New(cmd, command.WithEnvVars(state.Env)))
		merr = multierror.Append(merr, err)
		merr = multierror.Append(merr, ui.Append(stdout, stderr))
		if err := merr.ErrorOrNil(); err != nil {
			return ui, wrapErrWithDiagnostics(
				err, "command failed", fmt.Sprintf("running inline command failed: %s", err.Error()),
			)
		}
	}

	for _, path := range state.Scripts {
		script, err := tfile.Open(path)
		defer script.Close() // nolint: staticcheck
		if err != nil {
			return ui, wrapErrWithDiagnostics(
				err, "invalid configuration", "unable to open script file", "scripts",
			)
		}

		err = r.copyAndRun(ctx, ui, ssh, script, "script", state.Env)
		if err != nil {
			return ui, err
		}
	}

	if state.Content != "" {
		content := tfile.NewReader(state.Content)
		defer content.Close()

		err = r.copyAndRun(ctx, ui, ssh, content, "content", state.Env)
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
	dst := fmt.Sprintf("/tmp/%s.sh", sha)
	err = ssh.Copy(ctx, src, dst)
	if err != nil {
		return wrapErrWithDiagnostics(
			err, "command failed", "copying src file content to remote script",
		)
	}

	stdout, stderr, err := ssh.Run(ctx, command.New(fmt.Sprintf("chmod 0777 %s", dst), command.WithEnvVars(env)))
	merr = multierror.Append(merr, err)
	merr = multierror.Append(merr, ui.Append(stdout, stderr))
	if merr.ErrorOrNil() != nil {
		return wrapErrWithDiagnostics(
			merr.ErrorOrNil(), "command failed", fmt.Sprintf("running changing ownership on script: %s", merr.Error()),
		)
	}

	stdout, stderr, err = ssh.Run(ctx, command.New(dst, command.WithEnvVars(env)))
	merr = multierror.Append(merr, err)
	merr = multierror.Append(merr, ui.Append(stdout, stderr))
	if merr.ErrorOrNil() != nil {
		return wrapErrWithDiagnostics(
			merr.ErrorOrNil(), "command failed", fmt.Sprintf("executing script: %s", merr.Error()),
		)
	}

	stdout, stderr, err = ssh.Run(ctx, command.New(fmt.Sprintf("rm -f %s", dst), command.WithEnvVars(env)))
	merr = multierror.Append(merr, err)
	merr = multierror.Append(merr, ui.Append(stdout, stderr))
	if merr.ErrorOrNil() != nil {
		return wrapErrWithDiagnostics(
			merr.ErrorOrNil(), "command failed", fmt.Sprintf("removing script file: %s", merr.Error()),
		)
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
func (s *remoteExecStateV1) FromTerraform5Value(val tftypes.Value) error {
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

	if vals["transport"].IsKnown() {
		return s.Transport.FromTerraform5Value(vals["transport"])
	}

	return nil
}

// Terraform5Type is the file state tftypes.Type.
func (s *remoteExecStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":          tftypes.String,
		"sum":         tftypes.String,
		"stdout":      tftypes.String,
		"stderr":      tftypes.String,
		"environment": tftypes.Map{AttributeType: tftypes.String},
		"inline":      tftypes.List{ElementType: tftypes.String},
		"scripts":     tftypes.List{ElementType: tftypes.String},
		"content":     tftypes.String,
		"transport":   s.Transport.Terraform5Type(),
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (s *remoteExecStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":          tfMarshalStringValue(s.ID),
		"sum":         tfMarshalStringValue(s.Sum),
		"stdout":      tfMarshalStringValue(s.Stdout),
		"stderr":      tfMarshalStringValue(s.Stderr),
		"content":     tfMarshalStringOptionalValue(s.Content),
		"transport":   s.Transport.Terraform5Value(),
		"inline":      tfMarshalStringSlice(s.Inline),
		"scripts":     tfMarshalStringSlice(s.Scripts),
		"environment": tfMarshalStringMap(s.Env),
	})
}

func (s *remoteExecStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}

func (s *remoteExecStateV1) shouldDelete() bool {
	if s.Content == "" && len(s.Inline) == 0 && len(s.Scripts) == 0 {
		return true
	}

	return false
}

func (s *remoteExecStateV1) hasUnknownAttributes() bool {
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
