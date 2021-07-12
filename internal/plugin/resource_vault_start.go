package plugin

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/hashicorp/enos-provider/internal/flightcontrol/remoteflight"
	"github.com/hashicorp/enos-provider/internal/flightcontrol/remoteflight/vault"
	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	it "github.com/hashicorp/enos-provider/internal/transport"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type vaultStart struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resourcerouter.Resource = (*vaultStart)(nil)

type vaultStartStateV1 struct {
	ID              string
	BinPath         string
	Config          *vaultConfig
	ConfigDir       string
	License         string
	Status          *tfNum
	SystemdUnitName string
	Transport       *embeddedTransportV1
	Username        string
}

type vaultConfig struct {
	APIAddr     string
	ClusterAddr string
	Listener    *vaultConfigBlock
	Storage     *vaultConfigBlock
	Seal        *vaultConfigBlock
	UI          *tfBool
}

type vaultConfigBlock struct {
	Type          string
	Attrs         map[string]interface{}
	OptionalAttrs map[string]struct{}
}

var _ State = (*vaultStartStateV1)(nil)

func newVaultStart() *vaultStart {
	return &vaultStart{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newVaultStartStateV1() *vaultStartStateV1 {
	return &vaultStartStateV1{
		Status:    &tfNum{},
		Transport: newEmbeddedTransport(),
		Config: &vaultConfig{
			Listener: &vaultConfigBlock{},
			Seal:     &vaultConfigBlock{},
			Storage:  &vaultConfigBlock{},
			UI:       &tfBool{},
		},
	}
}

func (r *vaultStart) Name() string {
	return "enos_vault_start"
}

func (r *vaultStart) Schema() *tfprotov5.Schema {
	return newVaultStartStateV1().Schema()
}

func (r *vaultStart) SetProviderConfig(meta tftypes.Value) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.FromTerraform5Value(meta)
}

func (r *vaultStart) GetProviderConfig() (*config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.Copy()
}

// ValidateResourceTypeConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (r *vaultStart) ValidateResourceTypeConfig(ctx context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	newState := newVaultStartStateV1()

	return transportUtil.ValidateResourceTypeConfig(ctx, newState, req)
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
func (r *vaultStart) UpgradeResourceState(ctx context.Context, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
	newState := newVaultStartStateV1()

	return transportUtil.UpgradeResourceState(ctx, newState, req)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *vaultStart) ReadResource(ctx context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	res := &tfprotov5.ReadResourceResponse{
		Diagnostics: []*tfprotov5.Diagnostic{},
	}

	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(ctx.Err()))
		return res, ctx.Err()
	default:
	}

	state := newVaultStartStateV1()
	err := unmarshal(state, req.CurrentState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	// It's possible that Terraform is calling this to read the resource during
	// a subsequent run. In that case, it's also possible that our service
	// status has changed. If we have enough configuration to build an SSH
	// client we should attempt to get the status.
	buildSSHClient := func() (it.Transport, error) {
		stateTransport := state.EmbeddedTransport()
		err = stateTransport.FromPrivate(req.Private)
		if err != nil {
			return nil, err
		}

		providerConfig, err := r.GetProviderConfig()
		if err != nil {
			return nil, err
		}

		transport, err := providerConfig.Transport.Copy()
		if err != nil {
			return nil, err
		}

		err = stateTransport.MergeInto(transport)
		if err != nil {
			return nil, err
		}

		return stateTransport.Client(ctx)
	}

	ssh, err := buildSSHClient()
	if err == nil {
		if state.BinPath != "" && state.BinPath != UnknownString {
			code, err := vault.Status(ctx, ssh, vault.NewStatusRequest(
				vault.WithStatusRequestBinPath(state.BinPath),
				vault.WithStatusRequestVaultAddr(state.Config.APIAddr),
			))
			if err == nil {
				state.Status.Set(int(code))
			}
		}
	}

	res.NewState, err = marshal(state)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	res.Private, err = state.EmbeddedTransport().ToPrivate()
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
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
func (r *vaultStart) ImportResourceState(ctx context.Context, req *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
	newState := newVaultStartStateV1()

	return transportUtil.ImportResourceState(ctx, newState, req)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *vaultStart) PlanResourceChange(ctx context.Context, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, error) {
	priorState := newVaultStartStateV1()
	proposedState := newVaultStartStateV1()

	res, transport, err := transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, r, req)
	if err != nil {
		return res, err
	}

	// When we're planning we need to determine if we've already applied before
	// or if we're planning to apply. If we already have an ID we've been applied
	// before and can simply plan to have the same state since it'll be a no-op
	// apply. If we haven't applied then we need to set all of our computed
	// outputs to unknown values.
	if priorState.ID == "" {
		proposedState.Status.unknown = true
	}

	err = transportUtil.PlanMarshalPlannedState(ctx, res, proposedState, transport)

	return res, err
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *vaultStart) ApplyResourceChange(ctx context.Context, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
	priorState := newVaultStartStateV1()
	plannedState := newVaultStartStateV1()

	res, err := transportUtil.ApplyUnmarshalState(ctx, priorState, plannedState, req)
	if err != nil {
		return res, err
	}

	// Check if the planned state attributes are blank. If they are then you
	// should delete the resource.
	if plannedState.BinPath == "" {
		// Delete the resource
		res.NewState, err = marshalDelete(plannedState)

		return res, err
	}

	transport, err := transportUtil.ApplyValidatePlannedAndBuildTransport(ctx, res, plannedState, r)
	if err != nil {
		return res, err
	}

	plannedID := "static"
	plannedState.ID = plannedID

	ssh, err := transport.Client(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}
	defer ssh.Close() //nolint: staticcheck

	// If our priorState ID is blank then we're creating the resource
	if priorState.ID == "" {
		err = plannedState.startVault(ctx, ssh)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
	} else if reflect.DeepEqual(plannedState, priorState) {
		err = plannedState.startVault(ctx, ssh)

		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(fmt.Errorf("%s", err)))
			return res, err
		}
	}

	err = transportUtil.ApplyMarshalNewState(ctx, res, plannedState, transport)

	return res, err
}

// Schema is the file states Terraform schema.
func (s *vaultStartStateV1) Schema() *tfprotov5.Schema {
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
					Name:     "bin_path", // where the vault binary is
					Type:     tftypes.String,
					Required: true,
				},
				{
					Name:     "config",
					Type:     s.Config.Terraform5Type(),
					Required: true,
				},
				{
					Name:     "config_dir", // where to write vault config
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:      "license", // the vault license
					Type:      tftypes.String,
					Optional:  true,
					Sensitive: true,
				},
				{
					Name: "status", // the vault status code
					// 0 - Initialized, Unsealed
					// 1 - Error
					// 2 - Sealed
					// 9 - Unknown - we couldn't get the status
					Type:     tftypes.Number,
					Computed: true,
				},
				{
					Name:     "unit_name", // sysmted unit name
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "username", // vault username
					Type:     tftypes.String,
					Optional: true,
				},
				s.Transport.SchemaAttributeTransport(),
			},
		},
	}
}

// Validate validates the configuration. This will validate the source file
// exists and that the transport configuration is valid.
func (s *vaultStartStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if s.BinPath == "" {
		return newErrWithDiagnostics("invalid configuration", "you must provide a vault binary path", "attribute")
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *vaultStartStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"bin_path":   &s.BinPath,
		"config_dir": &s.ConfigDir,
		"id":         &s.ID,
		"license":    &s.License,
		"status":     s.Status,
		"unit_name":  &s.SystemdUnitName,
		"username":   &s.Username,
	})
	if err != nil {
		return err
	}

	if vals["config"].IsKnown() {
		err = s.Config.FromTerraform5Value(vals["config"])
		if err != nil {
			return err
		}
	}

	if vals["transport"].IsKnown() {
		err = s.Transport.FromTerraform5Value(vals["transport"])
		if err != nil {
			return err
		}
	}

	return nil
}

// Terraform5Type is the file state tftypes.Type.
func (s *vaultStartStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"bin_path":   tftypes.String,
		"config":     s.Config.Terraform5Type(),
		"config_dir": tftypes.String,
		"id":         tftypes.String,
		"license":    tftypes.String,
		"status":     s.Status.TFType(),
		"unit_name":  tftypes.String,
		"transport":  s.Transport.Terraform5Type(),
		"username":   tftypes.String,
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (s *vaultStartStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"bin_path":   tfMarshalStringValue(s.BinPath),
		"config":     s.Config.Terraform5Value(),
		"config_dir": tfMarshalStringOptionalValue(s.ConfigDir),
		"id":         tfMarshalStringValue(s.ID),
		"license":    tfMarshalStringOptionalValue(s.License),
		"status":     s.Status.TFValue(),
		"unit_name":  tfMarshalStringOptionalValue(s.SystemdUnitName),
		"transport":  s.Transport.Terraform5Value(),
		"username":   tfMarshalStringOptionalValue(s.Username),
	})
}

// EmbeddedTransport returns a pointer the resources embedded transport.
func (s *vaultStartStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}

// FromTerraform5Value unmarshals the value to the struct
func (s *vaultConfigBlock) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"type": &s.Type,
	})
	if err != nil {
		return err
	}

	attrs, ok := vals["attributes"]
	if ok {
		s.Attrs, err = tfUnmarshalDynamicPsuedoType(attrs)
		if err != nil {
			return err
		}
	}

	return nil
}

// Terraform5Type is the tftypes.Type
func (s *vaultConfigBlock) Terraform5Type() tftypes.Type {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"type":       tftypes.String,
			"attributes": tftypes.DynamicPseudoType,
		},
	}
}

// Terraform5Type is the tftypes.Value
func (s *vaultConfigBlock) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"type":       tfMarshalStringValue(s.Type),
		"attributes": tfMarshalDynamicPsuedoTypeObject(s.Attrs, s.OptionalAttrs),
	})
}

func (c *vaultConfig) Terraform5Type() tftypes.Type {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"api_addr":     tftypes.String,
			"cluster_addr": tftypes.String,
			"listener":     c.Listener.Terraform5Type(),
			"storage":      c.Storage.Terraform5Type(),
			"seal":         c.Seal.Terraform5Type(),
			"ui":           c.UI.TFType(),
		},
	}
}

func (c *vaultConfig) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(c.Terraform5Type(), map[string]tftypes.Value{
		"api_addr":     tfMarshalStringValue(c.APIAddr),
		"cluster_addr": tfMarshalStringValue(c.ClusterAddr),
		"listener":     c.Listener.Terraform5Value(),
		"seal":         c.Seal.Terraform5Value(),
		"storage":      c.Storage.Terraform5Value(),
		"ui":           c.UI.TFValue(),
	})
}

// FromTerraform5Value unmarshals the value to the struct
func (c *vaultConfig) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"api_addr":     &c.APIAddr,
		"cluster_addr": &c.ClusterAddr,
		"ui":           c.UI,
	})
	if err != nil {
		return err
	}

	listener, ok := vals["listener"]
	if ok {
		err = c.Listener.FromTerraform5Value(listener)
		if err != nil {
			return err
		}
	}

	seal, ok := vals["seal"]
	if ok {
		err = c.Seal.FromTerraform5Value(seal)
		if err != nil {
			return err
		}
	}

	storage, ok := vals["storage"]
	if ok {
		err = c.Storage.FromTerraform5Value(storage)
		if err != nil {
			return err
		}
	}

	return nil
}

// ToHCLConfig returns the vault config in the remoteflight HCLConfig format
func (c *vaultConfig) ToHCLConfig() *vault.HCLConfig {
	hclConfig := &vault.HCLConfig{
		APIAddr:     c.APIAddr,
		ClusterAddr: c.ClusterAddr,
		Listener: &vault.HCLBlock{
			Label: c.Listener.Type,
			Attrs: c.Listener.Attrs,
		},
		Seal: &vault.HCLBlock{
			Label: c.Seal.Type,
			Attrs: c.Seal.Attrs,
		},
		Storage: &vault.HCLBlock{
			Label: c.Storage.Type,
			Attrs: c.Storage.Attrs,
		},
	}

	ui, ok := c.UI.Get()
	if ok {
		hclConfig.UI = ui
	}

	return hclConfig
}

func (s *vaultStartStateV1) startVault(ctx context.Context, ssh it.Transport) error {
	var err error

	// Set the status to unknown. After we start vault and wait for it to be running
	// we'll update the status again.
	s.Status.Set(int(vault.StatusUnknown))

	// Ensure that the vault user is created
	vaultUsername := "vault"
	if s.Username != "" && s.Username != UnknownString {
		vaultUsername = s.Username
	}

	configDir := "/etc/vault.d"
	if s.ConfigDir != "" && s.ConfigDir != UnknownString {
		configDir = s.ConfigDir
	}

	_, err = remoteflight.FindOrCreateUser(ctx, ssh, remoteflight.NewUser(
		remoteflight.WithUserName(vaultUsername),
		remoteflight.WithUserHomeDir(configDir),
		remoteflight.WithUserShell("/bin/false"),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "vault user", "failed to find or create the vault user")
	}

	configFilePath := filepath.Join(configDir, "vault.hcl")
	unitName := "vault"
	if s.SystemdUnitName != "" && s.SystemdUnitName != UnknownString {
		unitName = s.SystemdUnitName
	}

	unit := remoteflight.SystemdUnit{
		"Unit": {
			"Description":           "HashiCorp Vault - A tool for managing secrets",
			"Documentation":         "https://www.vaultproject.io/docs/",
			"Requires":              "network-online.target",
			"After":                 "network-online.target",
			"ConditionFileNotEmpty": configFilePath,
			"StartLimitIntervalSec": "60",
			"StartLimitBurst":       "3",
		},
		"Service": {
			"User":                  "vault",
			"Group":                 "vault",
			"ProtectSystem":         "full",
			"ProtectHome":           "read-only",
			"PrivateTmp":            "yes",
			"PrivateDevices":        "yes",
			"SecureBits":            "keep-caps",
			"AmbientCapabilities":   "CAP_IPC_LOCK",
			"Capabilities":          "CAP_IPC_LOCK+ep",
			"CapabilityBoundingSet": "CAP_SYSLOG CAP_IPC_LOCK",
			"NoNewPrivileges":       "yes",
			"ExecStart":             fmt.Sprintf("%s server -config %s", s.BinPath, configFilePath),
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

	if s.License != "" && s.License != UnknownString {
		licensePath := filepath.Join(s.ConfigDir, "vault.lic")
		err = remoteflight.CopyFile(ctx, ssh, remoteflight.NewCopyFileRequest(
			remoteflight.WithCopyFileDestination(licensePath),
			remoteflight.WithCopyFileChmod("640"),
			remoteflight.WithCopyFileChown(fmt.Sprintf("%s:%s", vaultUsername, vaultUsername)),
			remoteflight.WithCopyFileContent(tfile.NewReader(s.License)),
		))

		if err != nil {
			return wrapErrWithDiagnostics(err, "vault license", "failed to copy vault license")
		}

		unit["Service"]["Environment"] = fmt.Sprintf("VAULT_LICENSE_PATH=%s", licensePath)
	}

	// Write the systemd unit
	err = remoteflight.CreateSystemdUnitFile(ctx, ssh, remoteflight.NewCreateSystemdUnitFileRequest(
		remoteflight.WithSystemdUnitUnitPath(fmt.Sprintf("/etc/systemd/system/%s.service", unitName)),
		remoteflight.WithSystemdUnitChmod("640"),
		remoteflight.WithSystemdUnitChown(fmt.Sprintf("%s:%s", vaultUsername, vaultUsername)),
		remoteflight.WithSystemdUnitFile(unit),
	))

	if err != nil {
		return wrapErrWithDiagnostics(err, "systemd unit", "failed to create the vault systemd unit")
	}

	// Create the vault HCL configuration file
	err = vault.CreateHCLConfigFile(ctx, ssh, vault.NewCreateHCLConfigFileRequest(
		vault.WithHCLConfigFilePath(configFilePath),
		vault.WithHCLConfigChmod("640"),
		vault.WithHCLConfigChown(fmt.Sprintf("%s:%s", vaultUsername, vaultUsername)),
		vault.WithHCLConfigFile(s.Config.ToHCLConfig()),
	))

	if err != nil {
		return wrapErrWithDiagnostics(err, "vault configuration", "failed to create the vault configuration file")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(1*time.Minute))
	defer cancel()

	// Restart the service and wait for it to be running
	err = vault.Restart(timeoutCtx, ssh, vault.NewStatusRequest(
		vault.WithStatusRequestBinPath(s.BinPath),
		vault.WithStatusRequestVaultAddr(s.Config.APIAddr),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "vault service", "failed to start the vault service")
	}

	err = vault.WaitForStatus(timeoutCtx, ssh, vault.NewStatusRequest(
		vault.WithStatusRequestBinPath(s.BinPath),
		vault.WithStatusRequestVaultAddr(s.Config.APIAddr),
	), vault.StatusInitializedUnsealed, vault.StatusSealed)
	if err != nil {
		return wrapErrWithDiagnostics(err, "vault service", "waiting for vault service")
	}

	code, err := vault.Status(timeoutCtx, ssh, vault.NewStatusRequest(
		vault.WithStatusRequestBinPath(s.BinPath),
		vault.WithStatusRequestVaultAddr(s.Config.APIAddr),
	))
	s.Status.Set(int(code))

	return err
}
