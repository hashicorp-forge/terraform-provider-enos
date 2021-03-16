package plugin

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tftypes"
)

type remoteExec struct {
	providerConfig *config
}

var _ resourcerouter.Resource = (*remoteExec)(nil)

type remoteExecStateV1 struct {
	ID        string
	Env       map[string]string
	Inline    []string
	Scripts   []string
	Sum       string
	Transport *embeddedTransportV1
}

var _ State = (*remoteExecStateV1)(nil)

func newRemoteExec() *remoteExec {
	return &remoteExec{
		providerConfig: newProviderConfig(),
	}
}

func newRemoteExecStateV1() *remoteExecStateV1 {
	return &remoteExecStateV1{
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
	return r.providerConfig.FromTerraform5Value(meta)
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

	res, transport, err := transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, *r.providerConfig.Transport, req)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	sha256, err := r.SHA256(ctx, proposedState)
	if err != nil {
		err = wrapErrWithDiagnostics(err, "invalid configuration", "unable to read all scripts", "scripts")
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}
	proposedState.Sum = sha256

	return transportUtil.PlanMarshalPlannedState(ctx, proposedState, transport)
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

	if len(plannedState.Inline) == 0 && len(plannedState.Scripts) == 0 {
		// Delete the resource
		res.NewState, err = marshalDelete(plannedState)
		return res, err
	}
	plannedState.ID = "static"

	res, transport, err := transportUtil.ApplyValidatePlannedAndBuildTransport(ctx, plannedState, *r.providerConfig.Transport)
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

		err = r.ExecuteCommands(ctx, plannedState, ssh)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
	}

	return transportUtil.ApplyMarshalNewState(ctx, plannedState, transport)
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

	for _, cmd := range state.Inline {
		ag.WriteString(command.SHA256(command.New(cmd, command.WithEnvVars(state.Env))))
	}

	var sha string
	var err error
	for _, path := range state.Scripts {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		sha, err = tfile.SHA256(path)
		if err != nil {
			return "", wrapErrWithDiagnostics(
				err, "invalid configuration", "unable to open script file", "scripts",
			)
		}

		ag.WriteString(sha)
	}

	return fmt.Sprintf("%x", sha256.Sum256([]byte(ag.String()))), nil
}

// ExecuteCommands executes any commands or scripts.
func (r *remoteExec) ExecuteCommands(ctx context.Context, state *remoteExecStateV1, ssh it.Transport) error {
	var stderr string
	for _, cmd := range state.Inline {
		select {
		case <-ctx.Done():
			return wrapErrWithDiagnostics(
				ctx.Err(), "timed out", "context deadline exceeded while running inline commands",
			)
		default:
		}

		_, stderr, err := ssh.Run(ctx, command.New(cmd, command.WithEnvVars(state.Env)))
		if err != nil {
			return wrapErrWithDiagnostics(
				err, "command failed", fmt.Sprintf("running inline command failed: %s", stderr),
			)
		}
	}

	var sha string
	var script it.Copyable
	var dst string
	var err error

	for _, path := range state.Scripts {
		select {
		case <-ctx.Done():
			return wrapErrWithDiagnostics(
				ctx.Err(), "timed out", "context deadline exceeded while running scripts",
			)
		default:
		}

		sha, err = tfile.SHA256(path)
		if err != nil {
			return wrapErrWithDiagnostics(
				err, "invalid configuration", "unable to open script file", "scripts",
			)
		}

		script, err = tfile.Open(path)
		defer script.Close() // nolint: staticcheck
		if err != nil {
			return wrapErrWithDiagnostics(
				err, "invalid configuration", "unable to open script file", "scripts",
			)
		}

		// TODO: Eventually we'll probably have to support /tmp being mounted
		// with no exec. In those cases we'll have to make this configurable
		// or find another strategy for executing scripts.
		dst = fmt.Sprintf("/tmp/%s.sh", sha)
		err = ssh.Copy(ctx, script, dst)
		if err != nil {
			return wrapErrWithDiagnostics(
				err, "command failed", fmt.Sprintf("running inline command failed: %s", stderr),
			)
		}

		_, stderr, err = ssh.Run(ctx, command.New(fmt.Sprintf("chmod 0777 %s", dst), command.WithEnvVars(state.Env)))
		if err != nil {
			return wrapErrWithDiagnostics(
				err, "command failed", fmt.Sprintf("running changing ownership on script: %s", stderr),
			)
		}

		_, stderr, err = ssh.Run(ctx, command.New(fmt.Sprintf("bash %s", dst), command.WithEnvVars(state.Env)))
		if err != nil {
			return wrapErrWithDiagnostics(
				err, "command failed", fmt.Sprintf("running executing script: %s", stderr),
			)
		}

		_, stderr, err = ssh.Run(ctx, command.New(fmt.Sprintf("rm %s", dst), command.WithEnvVars(state.Env)))
		if err != nil {
			return wrapErrWithDiagnostics(
				err, "command failed", fmt.Sprintf("running executing script: %s", stderr),
			)
		}
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

	// Make sure that we have either an inline or script command
	if len(s.Inline) == 0 && len(s.Scripts) == 0 {
		return newErrWithDiagnostics("invalid configuration", "you must provide either inline commands or scripts", "inline")
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
		"id":  &s.ID,
		"sum": &s.Sum,
	})
	if err != nil {
		return err
	}

	if vals["environment"].IsKnown() && !vals["environment"].IsNull() {
		s.Env, err = tfUnmarshalStringMap(vals["environment"])
		if err != nil {
			return err
		}
	}

	if vals["inline"].IsKnown() && !vals["inline"].IsNull() {
		s.Inline, err = tfUnmarshalStringSlice(vals["inline"])
		if err != nil {
			return err
		}
	}

	if vals["scripts"].IsKnown() && !vals["scripts"].IsNull() {
		s.Scripts, err = tfUnmarshalStringSlice(vals["scripts"])
		if err != nil {
			return err
		}
	}

	if !vals["transport"].IsKnown() {
		return nil
	}

	return s.Transport.FromTerraform5Value(vals["transport"])
}

// Terraform5Type is the file state tftypes.Type.
func (s *remoteExecStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":          tftypes.String,
		"sum":         tftypes.String,
		"environment": tftypes.Map{AttributeType: tftypes.String},
		"inline":      tftypes.List{ElementType: tftypes.String},
		"scripts":     tftypes.List{ElementType: tftypes.String},
		"transport":   s.Transport.Terraform5Type(),
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (s *remoteExecStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":          tfMarshalStringValue(s.ID),
		"sum":         tfMarshalStringValue(s.Sum),
		"transport":   s.Transport.Terraform5Value(),
		"inline":      tfMarshalStringSlice(s.Inline),
		"scripts":     tfMarshalStringSlice(s.Scripts),
		"environment": tfMarshalStringMap(s.Env),
	})
}

func (s *remoteExecStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}
