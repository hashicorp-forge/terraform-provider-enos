package plugin

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/hashicorp/enos-provider/internal/diags"
	"github.com/hashicorp/enos-provider/internal/remoteflight"
	"github.com/hashicorp/enos-provider/internal/remoteflight/consul"
	"github.com/hashicorp/enos-provider/internal/remoteflight/hcl"
	resource "github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/enos-provider/internal/server/state"
	it "github.com/hashicorp/enos-provider/internal/transport"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type consulStart struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resource.Resource = (*consulStart)(nil)

type consulStartStateV1 struct {
	ID              *tfString
	BinPath         *tfString
	ConfigDir       *tfString
	DataDir         *tfString
	Config          *consulConfig
	License         *tfString
	SystemdUnitName *tfString
	Transport       *embeddedTransportV1
	Username        *tfString
}

type consulConfig struct {
	Datacenter      *tfString
	DataDir         *tfString
	RetryJoin       *tfStringSlice
	Server          *tfBool
	BootstrapExpect *tfNum
	LogFile         *tfString
	LogLevel        *tfString
}

var _ state.State = (*consulStartStateV1)(nil)

func newConsulStart() *consulStart {
	return &consulStart{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newConsulStartStateV1() *consulStartStateV1 {
	return &consulStartStateV1{
		ID:        newTfString(),
		BinPath:   newTfString(),
		ConfigDir: newTfString(),
		DataDir:   newTfString(),
		Config: &consulConfig{
			Datacenter:      newTfString(),
			DataDir:         newTfString(),
			RetryJoin:       newTfStringSlice(),
			Server:          newTfBool(),
			BootstrapExpect: newTfNum(),
			LogFile:         newTfString(),
			LogLevel:        newTfString(),
		},
		License:         newTfString(),
		SystemdUnitName: newTfString(),
		Transport:       newEmbeddedTransport(),
		Username:        newTfString(),
	}
}

func (r *consulStart) Name() string {
	return "enos_consul_start"
}

func (r *consulStart) Schema() *tfprotov6.Schema {
	return newConsulStartStateV1().Schema()
}

func (r *consulStart) SetProviderConfig(meta tftypes.Value) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.FromTerraform5Value(meta)
}

func (r *consulStart) GetProviderConfig() (*config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.Copy()
}

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (r *consulStart) ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	newState := newConsulStartStateV1()

	transportUtil.ValidateResourceConfig(ctx, newState, req, res)
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
//
// Upgrading the resource state generally goes as follows:
//
//  1. Unmarshal the RawState to the corresponding tftypes.Value that matches
//     schema version of the state we're upgrading from.
//  2. Create a new tftypes.Value for the current state and migrate the old
//     values to the new values.
//  3. Upgrade the existing state with the new values and return the marshaled
//     version of the current upgraded state.
func (r *consulStart) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	newState := newConsulStartStateV1()

	transportUtil.UpgradeResourceState(ctx, newState, req, res)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *consulStart) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	newState := newConsulStartStateV1()

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
func (r *consulStart) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	newState := newConsulStartStateV1()

	transportUtil.ImportResourceState(ctx, newState, req, res)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *consulStart) PlanResourceChange(ctx context.Context, req resource.PlanResourceChangeRequest, res *resource.PlanResourceChangeResponse) {
	priorState := newConsulStartStateV1()
	proposedState := newConsulStartStateV1()
	res.PlannedState = proposedState

	transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, r, req, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	if _, ok := priorState.ID.Get(); !ok {
		proposedState.ID.Unknown = true
	}
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *consulStart) ApplyResourceChange(ctx context.Context, req resource.ApplyResourceChangeRequest, res *resource.ApplyResourceChangeResponse) {
	priorState := newConsulStartStateV1()
	plannedState := newConsulStartStateV1()
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
	defer client.Close() //nolint: staticcheck

	// If our priorState ID is blank then we're creating the resource
	if _, ok := priorState.ID.Get(); !ok {
		err = plannedState.startConsul(ctx, client)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Consul Start Error", err))
			return
		}
	} else if reflect.DeepEqual(plannedState, priorState) {
		err = plannedState.startConsul(ctx, client)

		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Consul Start Error", err))
			return
		}
	}
}

// Schema is the file states Terraform schema.
func (s *consulStartStateV1) Schema() *tfprotov6.Schema {
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
					Name:     "bin_path", // where the consul binary is
					Type:     tftypes.String,
					Required: true,
				},
				{
					Name:     "config",
					Type:     s.Config.Terraform5Type(),
					Optional: true,
				},
				{
					Name:     "config_dir", // where to write consul config
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "data_dir", // where to write consul data
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:      "license", // the consul license
					Type:      tftypes.String,
					Optional:  true,
					Sensitive: true,
				},
				{
					Name:     "unit_name", // sysmted unit name
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "username", // consul username
					Type:     tftypes.String,
					Optional: true,
				},
				s.Transport.SchemaAttributeTransport(),
			},
		},
	}
}

// Schema is the file states Terraform schema.
func (c *consulConfig) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:     "datacenter",
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "data_dir",
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "retry_join",
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "server",
					Type:     tftypes.Bool,
					Optional: true,
				},
				{
					Name:     "bootstrap_expect",
					Type:     tftypes.Number,
					Optional: true,
				},
				{
					Name:     "log_file",
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "log_level",
					Type:     tftypes.String,
					Optional: true,
				},
			},
		},
	}
}

// Validate validates the configuration. This will validate the source file
// exists and that the transport configuration is valid.
func (s *consulStartStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := checkK8STransportNotConfigured(s, "enos_consul_start"); err != nil {
		return err
	}

	if _, ok := s.BinPath.Get(); !ok {
		return ValidationError("you must provide a consul binary path", "attribute")
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Consul with As().
func (s *consulStartStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"bin_path":   s.BinPath,
		"config_dir": s.ConfigDir,
		"data_dir":   s.DataDir,
		"id":         s.ID,
		"license":    s.License,
		"unit_name":  s.SystemdUnitName,
		"username":   s.Username,
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

	if !vals["transport"].IsKnown() {
		return nil
	}

	return s.Transport.FromTerraform5Value(vals["transport"])
}

// Terraform5Type is the file state tftypes.Type.
func (s *consulStartStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"bin_path":   s.BinPath.TFType(),
		"config":     s.Config.Terraform5Type(),
		"data_dir":   s.DataDir.TFType(),
		"config_dir": s.ConfigDir.TFType(),
		"id":         s.ID.TFType(),
		"license":    s.License.TFType(),
		"unit_name":  s.SystemdUnitName.TFType(),
		"transport":  s.Transport.Terraform5Type(),
		"username":   s.Username.TFType(),
	}}
}

// Terraform5Value is the file state tftypes.Value.
func (s *consulStartStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"bin_path":   s.BinPath.TFValue(),
		"config":     s.Config.Terraform5Value(),
		"data_dir":   s.DataDir.TFValue(),
		"config_dir": s.ConfigDir.TFValue(),
		"id":         s.ID.TFValue(),
		"license":    s.License.TFValue(),
		"unit_name":  s.SystemdUnitName.TFValue(),
		"transport":  s.Transport.Terraform5Value(),
		"username":   s.Username.TFValue(),
	})
}

// EmbeddedTransport returns a pointer the resources embedded transport.
func (s *consulStartStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}

func (c *consulConfig) Terraform5Type() tftypes.Type {
	return tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"data_dir":         c.DataDir.TFType(),
			"datacenter":       c.Datacenter.TFType(),
			"retry_join":       c.RetryJoin.TFType(),
			"server":           c.Server.TFType(),
			"bootstrap_expect": c.BootstrapExpect.TFType(),
			"log_file":         c.LogFile.TFType(),
			"log_level":        c.LogLevel.TFType(),
		},
	}
}

func (c *consulConfig) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(c.Terraform5Type(), map[string]tftypes.Value{
		"data_dir":         c.DataDir.TFValue(),
		"datacenter":       c.Datacenter.TFValue(),
		"retry_join":       c.RetryJoin.TFValue(),
		"server":           c.Server.TFValue(),
		"bootstrap_expect": c.BootstrapExpect.TFValue(),
		"log_file":         c.LogFile.TFValue(),
		"log_level":        c.LogLevel.TFValue(),
	})
}

// FromTerraform5Value unmarshals the value to the struct
func (c *consulConfig) FromTerraform5Value(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]interface{}{
		"data_dir":         c.DataDir,
		"datacenter":       c.Datacenter,
		"retry_join":       c.RetryJoin,
		"server":           c.Server,
		"bootstrap_expect": c.BootstrapExpect,
		"log_file":         c.LogFile,
		"log_level":        c.LogLevel,
	})
	if err != nil {
		return err
	}
	return nil
}

// ToHCLConfig returns the consul config in the remoteflight HCLConfig format
func (c *consulConfig) ToHCLConfig() *hcl.Builder {
	hlcBuilder := hcl.NewBuilder()

	if dataCenter, ok := c.Datacenter.Get(); ok {
		hlcBuilder.AppendAttribute("datacenter", dataCenter)
	}

	if dataDir, ok := c.DataDir.Get(); ok {
		hlcBuilder.AppendAttribute("data_dir", dataDir)
	}

	if retryJoin, ok := c.RetryJoin.GetStrings(); ok {
		hlcBuilder.AppendAttribute("retry_join", retryJoin)
	}

	if server, ok := c.Server.Get(); ok {
		hlcBuilder.AppendAttribute("server", server)
	}

	if bootstrapExpect, ok := c.BootstrapExpect.Get(); ok {
		hlcBuilder.AppendAttribute("bootstrap_expect", int64(bootstrapExpect))
	}

	if logFile, ok := c.LogFile.Get(); ok {
		hlcBuilder.AppendAttribute("log_file", logFile)
	}

	if logLevel, ok := c.LogLevel.Get(); ok {
		hlcBuilder.AppendAttribute("log_level", logLevel)
	}

	return hlcBuilder
}

func (s *consulStartStateV1) Debug() string {
	return s.EmbeddedTransport().Debug()
}

func (s *consulStartStateV1) startConsul(ctx context.Context, client it.Transport) error {
	var err error

	// Ensure that the consul user is created
	consulUsername := "consul"
	if user, ok := s.Username.Get(); ok {
		consulUsername = user
	}

	configDir := "/etc/consul.d"
	if cdir, ok := s.ConfigDir.Get(); ok {
		configDir = cdir
	}

	dataDir := "/opt/consul/data"
	if ddir, ok := s.DataDir.Get(); ok {
		dataDir = ddir
	}

	_, err = remoteflight.FindOrCreateUser(ctx, client, remoteflight.NewUser(
		remoteflight.WithUserName(consulUsername),
		remoteflight.WithUserHomeDir(dataDir),
		remoteflight.WithUserShell("/bin/false"),
	))
	if err != nil {
		return fmt.Errorf("failed to find or create the consul user, due to: %w", err)
	}

	configFilePath := filepath.Join(configDir, "consul.hcl")

	unitName := "consul"
	if unit, ok := s.SystemdUnitName.Get(); ok {
		unitName = unit
	}

	//nolint:typecheck // Temporarily ignore typecheck linting error: missing type in composite literal
	unit := remoteflight.SystemdUnit{
		"Unit": {
			"Description":           "HashiCorp Consul - A service mesh solution",
			"Documentation":         "https://www.consul.io/",
			"Requires":              "network-online.target",
			"After":                 "network-online.target",
			"ConditionFileNotEmpty": configFilePath,
		},
		"Service": {
			"Type":          "notify",
			"User":          "root",
			"Group":         "root",
			"ProtectSystem": "full",
			"ExecStart":     fmt.Sprintf("%s agent -config-dir %s", s.BinPath.Value(), configFilePath),
			"ExecReload":    "/bin/kill --signal HUP $MAINPID",
			"KillMode":      "process",
			"KillSignal":    "SIGINT",
			"Restart":       "on-failure",
			"LimitNOFILE":   "65536",
		},
		"Install": {
			"WantedBy": "multi-user.target",
		},
	}

	if license, ok := s.License.Get(); ok {
		licensePath := filepath.Join(configDir, "consul.lic")
		err = remoteflight.CopyFile(ctx, client, remoteflight.NewCopyFileRequest(
			remoteflight.WithCopyFileDestination(licensePath),
			remoteflight.WithCopyFileChmod("644"),
			remoteflight.WithCopyFileChown(fmt.Sprintf("%s:%s", consulUsername, consulUsername)),
			remoteflight.WithCopyFileContent(tfile.NewReader(license)),
		))

		if err != nil {
			return fmt.Errorf("failed to copy consul license, due to: %w", err)
		}

		// Validate the Consul license file
		err = consul.ValidateConsulLicense(ctx, client, consul.NewValidateFileRequest(
			consul.WithValidateConfigBinPath(s.BinPath.Value()),
			consul.WithValidateFilePath(licensePath),
		))

		if err != nil {
			return fmt.Errorf("consul license validation failed, due to: %w", err)
		}

		unit["Service"]["Environment"] = fmt.Sprintf("CONSUL_LICENSE_PATH=%s", licensePath)
	}

	// Write the systemd unit
	err = remoteflight.CreateSystemdUnitFile(ctx, client, remoteflight.NewCreateSystemdUnitFileRequest(
		remoteflight.WithSystemdUnitUnitPath(fmt.Sprintf("/etc/systemd/system/%s.service", unitName)),
		remoteflight.WithSystemdUnitChmod("644"),
		remoteflight.WithSystemdUnitChown(fmt.Sprintf("%s:%s", consulUsername, consulUsername)),
		remoteflight.WithSystemdUnitFile(unit),
	))

	if err != nil {
		return fmt.Errorf("failed to create the consul systemd unit, due to: %w", err)
	}

	config := s.Config.ToHCLConfig()

	// Create the consul HCL configuration file
	err = hcl.CreateHCLConfigFile(ctx, client, hcl.NewCreateHCLConfigFileRequest(
		hcl.WithHCLConfigFilePath(configFilePath),
		hcl.WithHCLConfigChmod("644"),
		hcl.WithHCLConfigChown(fmt.Sprintf("%s:%s", consulUsername, consulUsername)),
		hcl.WithHCLConfigFile(config),
	))

	if err != nil {
		return fmt.Errorf("failed to create the consul configuration file, due to: %w", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(1*time.Minute))
	defer cancel()

	// Create the consul data directory
	err = remoteflight.CreateDirectory(ctx, client, remoteflight.NewCreateDirectoryRequest(
		remoteflight.WithDirName(dataDir),
		remoteflight.WithDirChown(consulUsername),
	))
	if err != nil {
		return fmt.Errorf("failed to change ownership on data directory, due to: %w", err)
	}

	// Create the consul config directory
	err = remoteflight.CreateDirectory(ctx, client, remoteflight.NewCreateDirectoryRequest(
		remoteflight.WithDirName(configDir),
		remoteflight.WithDirChown(consulUsername),
	))
	if err != nil {
		return fmt.Errorf("failed to change ownership on config directory, due to: %w", err)
	}

	// Validate the Consul config file
	err = consul.ValidateConsulConfig(ctx, client, consul.NewValidateFileRequest(
		consul.WithValidateConfigBinPath(s.BinPath.Value()),
		consul.WithValidateFilePath(configFilePath),
	))
	if err != nil {
		return fmt.Errorf("failed to validate consul configuration file, due to: %w", err)
	}

	// Restart the service and wait for it to be running
	err = consul.Restart(timeoutCtx, client, consul.NewStatusRequest(
		consul.WithStatusRequestBinPath(s.BinPath.Value()),
	))
	if err != nil {
		return fmt.Errorf("failed to start the consul service, due to: %w", err)
	}

	return err
}
