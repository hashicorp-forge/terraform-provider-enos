package plugin

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/enos-provider/internal/remoteflight/hcl"
	"github.com/hashicorp/enos-provider/internal/remoteflight/vault"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	it "github.com/hashicorp/enos-provider/internal/transport"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	defaultRaftDataDir = "/opt/raft/data"
	raftStorageType    = "raft"
)

type vaultStart struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resourcerouter.Resource = (*vaultStart)(nil)

type vaultStartStateV1 struct {
	ID              *tfString
	BinPath         *tfString
	Config          *vaultConfig
	ConfigDir       *tfString
	License         *tfString
	Status          *tfNum
	SystemdUnitName *tfString
	ManageService   *tfBool
	Transport       *embeddedTransportV1
	Username        *tfString
}

type vaultConfig struct {
	ClusterName *tfString
	APIAddr     *tfString
	ClusterAddr *tfString
	Listener    *vaultConfigBlock
	Storage     *vaultConfigBlock
	Seal        *vaultConfigBlock
	UI          *tfBool
}

type vaultConfigBlock struct {
	Type  *tfString
	Attrs *tfObject
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
		ID:      newTfString(),
		BinPath: newTfString(),
		Config: &vaultConfig{
			ClusterName: newTfString(),
			APIAddr:     newTfString(),
			ClusterAddr: newTfString(),
			Listener:    newVaultConfigBlock(),
			Seal:        newVaultConfigBlock(),
			Storage:     newVaultConfigBlock(),
			UI:          newTfBool(),
		},
		ConfigDir:       newTfString(),
		License:         newTfString(),
		Status:          newTfNum(),
		SystemdUnitName: newTfString(),
		ManageService:   newTfBool(),
		Transport:       newEmbeddedTransport(),
		Username:        newTfString(),
	}
}

func newVaultConfigBlock() *vaultConfigBlock {
	return &vaultConfigBlock{
		Type:  newTfString(),
		Attrs: newTfObject(),
	}
}

func (r *vaultStart) Name() string {
	return "enos_vault_start"
}

func (r *vaultStart) Schema() *tfprotov6.Schema {
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

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (r *vaultStart) ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	newState := newVaultStartStateV1()

	transportUtil.ValidateResourceConfig(ctx, newState, req, res)
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
func (r *vaultStart) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	newState := newVaultStartStateV1()

	transportUtil.UpgradeResourceState(ctx, newState, req, res)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *vaultStart) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	newState := newVaultStartStateV1()

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
func (r *vaultStart) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	newState := newVaultStartStateV1()

	transportUtil.ImportResourceState(ctx, newState, req, res)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *vaultStart) PlanResourceChange(ctx context.Context, req tfprotov6.PlanResourceChangeRequest, res *tfprotov6.PlanResourceChangeResponse) {
	priorState := newVaultStartStateV1()
	proposedState := newVaultStartStateV1()

	transport := transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, r, req, res)
	if hasErrors(res.Diagnostics) {
		return
	}

	// When we're planning we need to determine if we've already applied before
	// or if we're planning to apply. If we already have an ID we've been applied
	// before and can simply plan to have the same state since it'll be a no-op
	// apply. If we haven't applied then we need to set all of our computed
	// outputs to unknown values.
	if _, ok := priorState.ID.Get(); !ok {
		proposedState.ID.Unknown = true
		proposedState.Status.Unknown = true
	}

	transportUtil.PlanMarshalPlannedState(ctx, res, proposedState, transport)
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *vaultStart) ApplyResourceChange(ctx context.Context, req tfprotov6.ApplyResourceChangeRequest, res *tfprotov6.ApplyResourceChangeResponse) {
	priorState := newVaultStartStateV1()
	plannedState := newVaultStartStateV1()

	transportUtil.ApplyUnmarshalState(ctx, priorState, plannedState, req, res)
	if hasErrors(res.Diagnostics) {
		return
	}

	// Check if the planned state attributes are blank. If they are then you
	// should delete the resource.
	if _, ok := plannedState.BinPath.Get(); !ok {
		// Delete the resource
		newState, err := marshalDelete(plannedState)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		} else {
			res.NewState = newState
		}
		return
	}

	transport := transportUtil.ApplyValidatePlannedAndBuildTransport(ctx, plannedState, r, res)
	if hasErrors(res.Diagnostics) {
		return
	}

	plannedState.ID.Set("static")

	ssh, err := transport.Client(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}
	defer ssh.Close() //nolint: staticcheck

	// If our priorState ID is blank then we're creating the resource
	if _, ok := priorState.ID.Get(); !ok {
		err = plannedState.startVault(ctx, ssh)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return
		}
	} else if reflect.DeepEqual(plannedState, priorState) {
		err = plannedState.startVault(ctx, ssh)

		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(fmt.Errorf("%s", err)))
			return
		}
	}

	transportUtil.ApplyMarshalNewState(ctx, res, plannedState, transport)
}

// Schema is the file states Terraform schema.
func (s *vaultStartStateV1) Schema() *tfprotov6.Schema {
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
					Name:     "manage_service",
					Type:     tftypes.Bool,
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

	if _, ok := s.BinPath.Get(); !ok {
		return newErrWithDiagnostics("invalid configuration", "you must provide a vault binary path", "attribute")
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *vaultStartStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"bin_path":       s.BinPath,
		"config_dir":     s.ConfigDir,
		"id":             s.ID,
		"license":        s.License,
		"status":         s.Status,
		"unit_name":      s.SystemdUnitName,
		"manage_service": s.ManageService,
		"username":       s.Username,
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
		"bin_path":       s.BinPath.TFType(),
		"config":         s.Config.Terraform5Type(),
		"config_dir":     s.ConfigDir.TFType(),
		"id":             s.ID.TFType(),
		"license":        s.License.TFType(),
		"status":         s.Status.TFType(),
		"unit_name":      s.SystemdUnitName.TFType(),
		"manage_service": s.ManageService.TFType(),
		"transport":      s.Transport.Terraform5Type(),
		"username":       s.Username.TFType(),
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (s *vaultStartStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"bin_path":       s.BinPath.TFValue(),
		"config":         s.Config.Terraform5Value(),
		"config_dir":     s.ConfigDir.TFValue(),
		"id":             s.ID.TFValue(),
		"license":        s.License.TFValue(),
		"status":         s.Status.TFValue(),
		"unit_name":      s.SystemdUnitName.TFValue(),
		"manage_service": s.ManageService.TFValue(),
		"transport":      s.Transport.Terraform5Value(),
		"username":       s.Username.TFValue(),
	})
}

// EmbeddedTransport returns a pointer the resources embedded transport.
func (s *vaultStartStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}

// FromTerraform5Value unmarshals the value to the struct
func (s *vaultConfigBlock) FromTerraform5Value(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]interface{}{
		"type":       s.Type,
		"attributes": s.Attrs,
	})

	return err
}

// Terraform5Type is the tftypes.Type
func (s *vaultConfigBlock) Terraform5Type() tftypes.Type {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"type":       s.Type.TFType(),
			"attributes": tftypes.DynamicPseudoType,
		},
	}
}

// Terraform5Type is the tftypes.Value
func (s *vaultConfigBlock) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"type":       s.Type.TFValue(),
		"attributes": s.Attrs.TFValue(),
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
			"cluster_name": c.ClusterName.TFType(),
		},
	}
}

func (c *vaultConfig) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(c.Terraform5Type(), map[string]tftypes.Value{
		"cluster_name": c.ClusterName.TFValue(),
		"api_addr":     c.APIAddr.TFValue(),
		"cluster_addr": c.ClusterAddr.TFValue(),
		"listener":     c.Listener.Terraform5Value(),
		"seal":         c.Seal.Terraform5Value(),
		"storage":      c.Storage.Terraform5Value(),
		"ui":           c.UI.TFValue(),
	})
}

// FromTerraform5Value unmarshals the value to the struct
func (c *vaultConfig) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"cluster_name": c.ClusterName,
		"api_addr":     c.APIAddr,
		"cluster_addr": c.ClusterAddr,
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
func (c *vaultConfig) ToHCLConfig() (*hcl.Builder, error) {
	hclBuilder := hcl.NewBuilder()

	if apiAddr, ok := c.APIAddr.Get(); ok {
		hclBuilder.AppendAttribute("api_addr", apiAddr)
	}

	if clusterAddr, ok := c.ClusterAddr.Get(); ok {
		hclBuilder.AppendAttribute("cluster_addr", clusterAddr)
	}

	if ui, ok := c.UI.Get(); ok {
		hclBuilder.AppendAttribute("ui", ui)
	}

	if label, ok := c.Listener.Type.Get(); ok {
		if attrs, ok := c.Listener.Attrs.GetObject(); ok {
			hclBuilder.AppendBlock("listener", []string{label}).AppendAttributes(attrs)
		}
	}

	if label, ok := c.Seal.Type.Get(); ok {
		if attrs, ok := c.Seal.Attrs.GetObject(); ok {
			hclBuilder.AppendBlock("seal", []string{label}).AppendAttributes(attrs)
		}
	}

	if storageLabel, ok := c.Storage.Type.Get(); ok {
		if attrs, ok := c.Storage.Attrs.GetObject(); ok {
			storageBlock := hclBuilder.AppendBlock("storage", []string{storageLabel}).AppendAttributes(attrs)

			if storageLabel == raftStorageType {
				storageBlock.AppendAttribute("path", defaultRaftDataDir)
				clusterName, ok := c.ClusterName.Get()
				if !ok {
					return nil, errors.New("ClusterName not found in Vault config")
				}
				storageBlock.AppendBlock("retry_join", []string{}).
					AppendAttribute("auto_join", fmt.Sprintf("provider=aws tag_key=Type tag_value=%s", clusterName)).
					AppendAttribute("auto_join_scheme", "http")
			}
		}
	}
	return hclBuilder, nil
}

func (s *vaultStartStateV1) startVault(ctx context.Context, ssh it.Transport) error {
	var err error

	// Set the status to unknown. After we start vault and wait for it to be running
	// we'll update the status again.
	s.Status.Set(int(vault.StatusUnknown))

	// Set up defaults
	vaultUsername := "vault"
	if user, ok := s.Username.Get(); ok {
		vaultUsername = user
	}

	configDir := "/etc/vault.d"
	if dir, ok := s.ConfigDir.Get(); ok {
		configDir = dir
	}
	configFilePath := filepath.Join(configDir, "vault.hcl")
	licensePath := filepath.Join(configDir, "vault.lic")

	envFilePath := "/etc/vault.d/vault.env"
	envFileContents := strings.Builder{}

	_, err = remoteflight.FindOrCreateUser(ctx, ssh, remoteflight.NewUser(
		remoteflight.WithUserName(vaultUsername),
		remoteflight.WithUserHomeDir(configDir),
		remoteflight.WithUserShell("/bin/false"),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "vault user", "failed to find or create the vault user")
	}

	// Copy the license file if we have one
	if license, ok := s.License.Get(); ok {
		err = remoteflight.CopyFile(ctx, ssh, remoteflight.NewCopyFileRequest(
			remoteflight.WithCopyFileDestination(licensePath),
			remoteflight.WithCopyFileChmod("640"),
			remoteflight.WithCopyFileChown(fmt.Sprintf("%s:%s", vaultUsername, vaultUsername)),
			remoteflight.WithCopyFileContent(tfile.NewReader(license)),
		))

		if err != nil {
			return wrapErrWithDiagnostics(err, "vault license", "failed to copy vault license")
		}

		envFileContents.WriteString(fmt.Sprintf("VAULT_LICENSE_PATH=%s\n", licensePath))
	}

	err = remoteflight.CopyFile(ctx, ssh, remoteflight.NewCopyFileRequest(
		remoteflight.WithCopyFileDestination(envFilePath),
		remoteflight.WithCopyFileChmod("644"),
		remoteflight.WithCopyFileChown(fmt.Sprintf("%s:%s", vaultUsername, vaultUsername)),
		remoteflight.WithCopyFileContent(tfile.NewReader(envFileContents.String())),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "vault environment", "failed to create the vault environment file")
	}

	// Create the vault HCL configuration file
	config, err := s.Config.ToHCLConfig()
	if err != nil {
		return wrapErrWithDiagnostics(err, "vault configuration", "failed to create the vault HCL configuration")
	}
	err = hcl.CreateHCLConfigFile(ctx, ssh, hcl.NewCreateHCLConfigFileRequest(
		hcl.WithHCLConfigFilePath(configFilePath),
		hcl.WithHCLConfigChmod("640"),
		hcl.WithHCLConfigChown(fmt.Sprintf("%s:%s", vaultUsername, vaultUsername)),
		hcl.WithHCLConfigFile(config),
	))

	if err != nil {
		return wrapErrWithDiagnostics(err, "vault configuration", "failed to create the vault configuration file")
	}

	// Manage the vault systemd service ourselves unless it has explicitly been
	// set that we should not.
	if manage, set := s.ManageService.Get(); !set || (set && manage) {
		unitName := "vault"
		if unit, ok := s.SystemdUnitName.Get(); ok {
			unitName = unit
		}

		//nolint:typecheck // Temporarily ignore typecheck linting error: missing type in composite literal
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
				"EnvironmentFile":       "/etc/vault.d/vault.env",
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
				"ExecStart":             fmt.Sprintf("%s server -config %s", s.BinPath.Value(), configFilePath),
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
		err = remoteflight.CreateSystemdUnitFile(ctx, ssh, remoteflight.NewCreateSystemdUnitFileRequest(
			remoteflight.WithSystemdUnitUnitPath(fmt.Sprintf("/etc/systemd/system/%s.service", unitName)),
			remoteflight.WithSystemdUnitChmod("640"),
			remoteflight.WithSystemdUnitChown(fmt.Sprintf("%s:%s", vaultUsername, vaultUsername)),
			remoteflight.WithSystemdUnitFile(unit),
		))

		if err != nil {
			return wrapErrWithDiagnostics(err, "systemd unit", "failed to create the vault systemd unit")
		}
	}

	if storageType, ok := s.Config.Storage.Type.Get(); ok && storageType == raftStorageType {
		err = remoteflight.CreateDirectory(ctx, ssh, remoteflight.NewCreateDirectoryRequest(
			remoteflight.WithDirName(defaultRaftDataDir),
			remoteflight.WithDirChown(vaultUsername),
		))
		if err != nil {
			return wrapErrWithDiagnostics(err, "raft data directory", "failed to change ownership on data directory")
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(1*time.Minute))
	defer cancel()

	// Restart the service and wait for it to be running
	err = vault.Restart(timeoutCtx, ssh, vault.NewStatusRequest(
		vault.WithStatusRequestBinPath(s.BinPath.Value()),
		vault.WithStatusRequestVaultAddr(s.Config.APIAddr.Value()),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "vault service", "failed to start the vault service")
	}

	err = vault.WaitForStatus(timeoutCtx, ssh, vault.NewStatusRequest(
		vault.WithStatusRequestBinPath(s.BinPath.Value()),
		vault.WithStatusRequestVaultAddr(s.Config.APIAddr.Value()),
	), vault.StatusInitializedUnsealed, vault.StatusSealed)
	if err != nil {
		return wrapErrWithDiagnostics(err, "vault service", "waiting for vault service")
	}

	code, err := vault.Status(timeoutCtx, ssh, vault.NewStatusRequest(
		vault.WithStatusRequestBinPath(s.BinPath.Value()),
		vault.WithStatusRequestVaultAddr(s.Config.APIAddr.Value()),
	))
	s.Status.Set(int(code))

	return err
}
