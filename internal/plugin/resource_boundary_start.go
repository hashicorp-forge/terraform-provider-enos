// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/diags"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/log"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight/systemd"
	resource "github.com/hashicorp-forge/terraform-provider-enos/internal/server/resourcerouter"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/state"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	tfile "github.com/hashicorp-forge/terraform-provider-enos/internal/transport/file"
)

type boundaryStart struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resource.Resource = (*boundaryStart)(nil)

type boundaryStartStateV1 struct {
	ID                   *tfString
	BinName              *tfString
	BinPath              *tfString
	ConfigPath           *tfString
	ConfigName           *tfString
	License              *tfString
	ManageService        *tfBool
	Status               *tfNum
	SystemdUnitName      *tfString
	Username             *tfString
	RecordingStoragePath *tfString
	Transport            *embeddedTransportV1
	failureHandlers
}

var _ state.State = (*boundaryStartStateV1)(nil)

func newBoundaryStart() *boundaryStart {
	return &boundaryStart{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newBoundaryStartStateV1() *boundaryStartStateV1 {
	transport := newEmbeddedTransport()
	fh := failureHandlers{
		TransportDebugFailureHandler(transport),
		GetApplicationLogsFailureHandler(transport, []string{"boundary"}),
	}

	return &boundaryStartStateV1{
		ID:                   newTfString(),
		BinName:              newTfString(),
		BinPath:              newTfString(),
		ConfigPath:           newTfString(),
		ConfigName:           newTfString(),
		ManageService:        newTfBool(),
		License:              newTfString(),
		Status:               newTfNum(),
		SystemdUnitName:      newTfString(),
		Username:             newTfString(),
		RecordingStoragePath: newTfString(),
		Transport:            transport,
		failureHandlers:      fh,
	}
}

func (r *boundaryStart) Name() string {
	return "enos_boundary_start"
}

func (r *boundaryStart) Schema() *tfprotov6.Schema {
	return newBoundaryStartStateV1().Schema()
}

func (r *boundaryStart) SetProviderConfig(meta tftypes.Value) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.FromTerraform5Value(meta)
}

func (r *boundaryStart) GetProviderConfig() (*config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.Copy()
}

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (r *boundaryStart) ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	newState := newBoundaryStartStateV1()

	transportUtil.ValidateResourceConfig(ctx, newState, req, res)
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (r *boundaryStart) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	newState := newBoundaryStartStateV1()

	transportUtil.UpgradeResourceState(ctx, newState, req, res)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *boundaryStart) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	newState := newBoundaryStartStateV1()

	transportUtil.ReadResource(ctx, newState, req, res)
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
func (r *boundaryStart) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	newState := newBoundaryStartStateV1()

	transportUtil.ImportResourceState(ctx, newState, req, res)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *boundaryStart) PlanResourceChange(ctx context.Context, req resource.PlanResourceChangeRequest, res *resource.PlanResourceChangeResponse) {
	priorState := newBoundaryStartStateV1()
	proposedState := newBoundaryStartStateV1()
	res.PlannedState = proposedState

	transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, r, req, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	if _, ok := priorState.ID.Get(); !ok {
		proposedState.ID.Unknown = true
		proposedState.Status.Unknown = true
	}
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *boundaryStart) ApplyResourceChange(ctx context.Context, req resource.ApplyResourceChangeRequest, res *resource.ApplyResourceChangeResponse) {
	priorState := newBoundaryStartStateV1()
	plannedState := newBoundaryStartStateV1()
	res.NewState = plannedState

	transportUtil.ApplyUnmarshalState(ctx, priorState, plannedState, req, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	// Check if the planned state attributes are blank. If they are then you
	// should delete the resource.
	if req.IsDelete() {
		// nothing to do on delete
		return
	}

	transport := transportUtil.ApplyValidatePlannedAndBuildTransport(ctx, plannedState, r, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	plannedState.ID.Set("static")

	client, err := transport.Client(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Transport Error", err))
		return
	}
	defer client.Close()

	// If our priorState ID is blank then we're creating the resource
	if _, ok := priorState.ID.Get(); !ok {
		err = plannedState.startBoundary(ctx, client)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Boundary Start Error", err))
			return
		}
	} else if reflect.DeepEqual(plannedState, priorState) {
		err = plannedState.startBoundary(ctx, client)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Boundary Start Error", err))
			return
		}
	}
}

// Schema is the file states Terraform schema.
func (s *boundaryStartStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			DescriptionKind: tfprotov6.StringKindMarkdown,
			Description: docCaretToBacktick(`
^enos_boundary_start^ is a resource that starts a Boundary cluster.

TODO(boundary) add an example for start
`),
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:        "id",
					Type:        tftypes.String,
					Computed:    true,
					Description: resourceStaticIDDescription,
				},
				{
					Name:        "bin_name",
					Type:        tftypes.String,
					Optional:    true,
					Description: "The name of boundary binary we're going to use when starting the cluster",
				},
				{
					Name:        "bin_path",
					Type:        tftypes.String,
					Required:    true,
					Description: "The path to the directory with binary we're going to use when starting the cluster",
				},
				{
					Name:        "config_path",
					Type:        tftypes.String,
					Required:    true,
					Description: "The path to the Boundary configuration to use when starting the cluster",
				},
				{
					Name:        "config_name",
					Type:        tftypes.String,
					Optional:    true,
					Description: "The name of a Boundary configuration to use when starting the cluster",
				},
				{
					Name:        "license",
					Type:        tftypes.String,
					Optional:    true,
					Sensitive:   true,
					Description: "The path to a license for Boundary Enterprise",
				},
				{
					Name:        "status",
					Type:        tftypes.Number,
					Computed:    true,
					Description: "The status code received when starting the cluster",
				},
				{
					Name:        "unit_name",
					Type:        tftypes.String,
					Optional:    true,
					Description: "The name of the systemd unit",
				},
				{
					Name:        "manage_service",
					Type:        tftypes.Bool,
					Optional:    true,
					Description: "Whether or not Enos should supply a systemd unit for the service",
				},
				{
					Name:        "username",
					Type:        tftypes.String,
					Optional:    true,
					Description: "The local username for the Boundary service",
				},
				{
					Name:        "recording_storage_path",
					Type:        tftypes.String,
					Optional:    true,
					Description: "The path to use for storage when recording",
				},
				s.Transport.SchemaAttributeTransport(supportsSSH),
			},
		},
	}
}

// Validate validates the configuration. This will validate the source file
// exists and that the transport configuration is valid.
func (s *boundaryStartStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := checkK8STransportNotConfigured(s, "enos_boundary_start"); err != nil {
		return err
	}

	if _, ok := s.ConfigPath.Get(); !ok {
		return ValidationError("you must provide the boundary config path", "config_path")
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Boundary with As().
func (s *boundaryStartStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id":                     s.ID,
		"bin_name":               s.BinName,
		"bin_path":               s.BinPath,
		"config_path":            s.ConfigPath,
		"config_name":            s.ConfigName,
		"manage_service":         s.ManageService,
		"license":                s.License,
		"status":                 s.Status,
		"unit_name":              s.SystemdUnitName,
		"username":               s.Username,
		"recording_storage_path": s.RecordingStoragePath,
	})
	if err != nil {
		return err
	}

	if !vals["transport"].IsKnown() {
		return nil
	}

	return s.Transport.FromTerraform5Value(vals["transport"])
}

// Terraform5Type is the file state tftypes.Type.
func (s *boundaryStartStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":                     s.ID.TFType(),
		"bin_name":               s.BinName.TFType(),
		"bin_path":               s.BinPath.TFType(),
		"config_path":            s.ConfigPath.TFType(),
		"config_name":            s.ConfigName.TFType(),
		"manage_service":         s.ManageService.TFType(),
		"license":                s.License.TFType(),
		"status":                 s.Status.TFType(),
		"unit_name":              s.SystemdUnitName.TFType(),
		"username":               s.Username.TFType(),
		"recording_storage_path": s.RecordingStoragePath.TFType(),

		"transport": s.Transport.Terraform5Type(),
	}}
}

// Terraform5Value is the file state tftypes.Value.
func (s *boundaryStartStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":                     s.ID.TFValue(),
		"bin_name":               s.BinName.TFValue(),
		"bin_path":               s.BinPath.TFValue(),
		"config_path":            s.ConfigPath.TFValue(),
		"config_name":            s.ConfigName.TFValue(),
		"manage_service":         s.ManageService.TFValue(),
		"license":                s.License.TFValue(),
		"status":                 s.Status.TFValue(),
		"unit_name":              s.SystemdUnitName.TFValue(),
		"username":               s.Username.TFValue(),
		"recording_storage_path": s.RecordingStoragePath.TFValue(),
		"transport":              s.Transport.Terraform5Value(),
	})
}

// EmbeddedTransport returns a pointer the resources embedded transport.
func (s *boundaryStartStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}

func (s *boundaryStartStateV1) startBoundary(ctx context.Context, transport it.Transport) error {
	var err error

	// defaults

	binName := "boundary"
	if name, ok := s.BinName.Get(); ok {
		binName = name
	}

	boundaryUser := "boundary"
	if user, ok := s.Username.Get(); ok {
		boundaryUser = user
	}

	configPath := "/etc/boundary"
	if path, ok := s.ConfigPath.Get(); ok {
		configPath = path
	}

	configName := "boundary.hcl"
	if name, ok := s.ConfigName.Get(); ok {
		configName = name
	}

	var envVars []string

	configFilePath := filepath.Join(configPath, configName)
	licensePath := filepath.Join(configPath, "boundary.lic")
	envFilePath := "/etc/boundary/boundary.env"

	// Create the OS user
	_, err = remoteflight.CreateOrUpdateUser(ctx, transport, remoteflight.NewUser(
		remoteflight.WithUserName(boundaryUser),
		remoteflight.WithUserHomeDir(configPath),
		remoteflight.WithUserShell("/bin/false"),
	))
	if err != nil {
		return fmt.Errorf("failed to find or create the boundary user, due to: %w", err)
	}

	// Create a directory for session recordings if a directory is given
	if path, ok := s.RecordingStoragePath.Get(); ok {
		err = remoteflight.CreateDirectory(ctx, transport, remoteflight.NewCreateDirectoryRequest(
			remoteflight.WithDirName(path),
			remoteflight.WithDirChown(fmt.Sprintf("%s:%s", boundaryUser, boundaryUser)),
		))
		if err != nil {
			return fmt.Errorf("failed to create recording_storage_path directory, due to %w", err)
		}
	}

	// Copy the license file if we have one
	if license, ok := s.License.Get(); ok {
		err = remoteflight.CopyFile(ctx, transport, remoteflight.NewCopyFileRequest(
			remoteflight.WithCopyFileDestination(licensePath),
			remoteflight.WithCopyFileChmod("640"),
			remoteflight.WithCopyFileChown(fmt.Sprintf("%s:%s", boundaryUser, boundaryUser)),
			remoteflight.WithCopyFileContent(tfile.NewReader(license)),
		))
		if err != nil {
			return fmt.Errorf("failed to copy boundary license, due to: %w", err)
		}

		envVars = append(envVars, fmt.Sprintf("BOUNDARY_LICENSE=file:///%s\n", licensePath))
	}

	// Copy the env file regardless, even if envVars empty, so the systemd unit is happy
	err = remoteflight.CopyFile(ctx, transport, remoteflight.NewCopyFileRequest(
		remoteflight.WithCopyFileDestination(envFilePath),
		remoteflight.WithCopyFileChmod("644"),
		remoteflight.WithCopyFileChown(fmt.Sprintf("%s:%s", boundaryUser, boundaryUser)),
		remoteflight.WithCopyFileContent(tfile.NewReader(strings.Join(envVars, "\n"))),
	))
	if err != nil {
		return fmt.Errorf("failed to create the boundary environment file, due to: %w", err)
	}

	sysd := systemd.NewClient(transport, log.NewLogger(ctx))

	// Manage the boundary systemd service ourselves unless it has explicitly been
	// set that we should not.
	if manage, set := s.ManageService.Get(); !set || (set && manage) {
		unitName := "boundary"
		if unit, ok := s.SystemdUnitName.Get(); ok {
			unitName = unit
		}

		unit := systemd.Unit{
			"Unit": {
				"Description":           "HashiCorp Boundary",
				"Documentation":         "https://www.boundaryproject.io/docs/",
				"Requires":              "network-online.target",
				"After":                 "network-online.target",
				"ConditionFileNotEmpty": configFilePath,
				"StartLimitIntervalSec": "60",
				"StartLimitBurst":       "3",
			},
			"Service": {
				"EnvironmentFile":       envFilePath,
				"User":                  boundaryUser,
				"Group":                 boundaryUser,
				"ProtectSystem":         "full",
				"ProtectHome":           "read-only",
				"PrivateTmp":            "yes",
				"PrivateDevices":        "yes",
				"SecureBits":            "keep-caps",
				"AmbientCapabilities":   "CAP_IPC_LOCK",
				"Capabilities":          "CAP_IPC_LOCK+ep",
				"CapabilityBoundingSet": "CAP_SYSLOG CAP_IPC_LOCK",
				"NoNewPrivileges":       "yes",
				"ExecStart":             fmt.Sprintf("%s/%s server -config %s", s.BinPath.Value(), binName, configFilePath),
				"ExecReload":            "/bin/kill --signal HUP $MAINPID",
				"KillMode":              "process",
				"KillSignal":            "SIGINT",
				"Restart":               "on-failure",
				"RestartSec":            "5",
				"TimeoutStopSec":        "30",
				"StartLimitInterval":    "60",
				"StartLimitIntervalSec": "60",
				"StartLimitBurst":       "10",
				"LimitNOFILE":           "65536",
				"LimitMEMLOCK":          "infinity",
			},
			"Install": {
				"WantedBy": "multi-user.target",
			},
		}

		// Write the systemd unit
		err = sysd.CreateUnitFile(ctx, systemd.NewCreateUnitFileRequest(
			systemd.WithUnitUnitPath(fmt.Sprintf("/etc/systemd/system/%s.service", unitName)),
			systemd.WithUnitChmod("640"),
			systemd.WithUnitChown(fmt.Sprintf("%s:%s", boundaryUser, boundaryUser)),
			systemd.WithUnitFile(unit),
		))
		if err != nil {
			return fmt.Errorf("failed to create the boundary systemd unit, due to: %w", err)
		}
	}

	err = sysd.RestartService(ctx, "boundary")
	if err != nil {
		return fmt.Errorf("failed to start the boundary service, due to: %w", err)
	}

	// set unknown values
	code := sysd.ServiceStatus(ctx, "boundary")
	s.Status.Set(int(code))

	return err
}
