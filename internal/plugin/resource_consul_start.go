// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/diags"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/log"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight/consul"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight/hcl"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight/systemd"
	resource "github.com/hashicorp-forge/terraform-provider-enos/internal/server/resourcerouter"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/state"
	istrings "github.com/hashicorp-forge/terraform-provider-enos/internal/strings"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	tfile "github.com/hashicorp-forge/terraform-provider-enos/internal/transport/file"
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

	failureHandlers
}

type consulConfig struct {
	BindAddr        *tfString
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
	transport := newEmbeddedTransport()
	fh := failureHandlers{
		TransportDebugFailureHandler(transport),
		GetApplicationLogsFailureHandler(transport, []string{"consul"}),
	}

	return &consulStartStateV1{
		ID:        newTfString(),
		BinPath:   newTfString(),
		ConfigDir: newTfString(),
		DataDir:   newTfString(),
		Config: &consulConfig{
			BindAddr:        newTfString(),
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
		Transport:       transport,
		Username:        newTfString(),
		failureHandlers: fh,
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
	defer client.Close()

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
			DescriptionKind: tfprotov6.StringKindMarkdown,
			Description: docCaretToBacktick(`
The ^enos_consul_start^ resource is capable of configuring a Consul service on a host. It handles creating the necessary configuration, configures licensing for Consul Enteprise, can manage systemd for install bundles, and starts the consul service.
`),
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:        "id",
					Type:        tftypes.String,
					Computed:    true,
					Description: resourceStaticIDDescription,
				},
				{
					Name:        "bin_path", // where the consul binary is
					Type:        tftypes.String,
					Required:    true,
					Description: "The fully qualified path to the Consul binary",
				},
				{
					Name:            "config",
					Type:            s.Config.Terraform5Type(),
					Optional:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description: docCaretToBacktick(`
- ^config.bind_addr^ (String) The Consul [bind_addr](https://developer.hashicorp.com/consul/docs/agent/config/config-files#bind_addr) value
- ^config.datacenter^ (String) The Consul [datacenter](https://developer.hashicorp.com/consul/docs/agent/config/config-files#datacenter) value
- ^config.data_dir^ (String) The Consul [data_dir](https://developer.hashicorp.com/consul/docs/agent/config/config-files#data_dir) value
- ^config.retry_join^ (List of String) The Consul [retry_join](https://developer.hashicorp.com/consul/docs/agent/config/config-files#retry_join) value
- ^config.bootstrap_expect^ (Number) The Consul [bootstrap_expect](https://developer.hashicorp.com/consul/docs/agent/config/config-files#bootstrap_expect) value
- ^config.server^ (Bool) The Consul [server](https://developer.hashicorp.com/consul/docs/agent/config/config-files#server_rpc_port) value
- ^config.log_file^ (String) The Consul [log_file](https://developer.hashicorp.com/consul/docs/agent/config/config-files#log_file) value
- ^config.log_level^ (String) The Consul [log_level](https://developer.hashicorp.com/consul/docs/agent/config/config-files#log_level) value
`),
				},
				{
					Name:        "config_dir", // where to write consul config
					Type:        tftypes.String,
					Optional:    true,
					Description: "The directory where the consul configuration resides",
				},
				{
					Name:        "data_dir", // where to write consul data
					Type:        tftypes.String,
					Optional:    true,
					Description: "The directory where Consul state will be stored",
				},
				{
					Name:        "license", // the consul license
					Type:        tftypes.String,
					Optional:    true,
					Sensitive:   true,
					Description: "A Consul Enterprise license. This is only required if you are starting a Consul Enterprise cluster",
				},
				{
					Name:        "unit_name", // sysmted unit name
					Type:        tftypes.String,
					Optional:    true,
					Description: "The name of the systemd unit to use",
				},
				{
					Name:        "username", // consul username
					Type:        tftypes.String,
					Optional:    true,
					Description: "The name of the local user for the consul service",
				},
				s.Transport.SchemaAttributeTransport(supportsSSH),
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
					Name:     "bind_addr",
					Type:     tftypes.String,
					Optional: true,
				},
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

func (c *consulConfig) Terraform5Type() tftypes.Type {
	return tftypes.Object{
		AttributeTypes:     c.attrs(),
		OptionalAttributes: c.optionalAttrs(),
	}
}

func (c *consulConfig) attrs() map[string]tftypes.Type {
	return map[string]tftypes.Type{
		"bind_addr":        c.BindAddr.TFType(),
		"datacenter":       c.Datacenter.TFType(),
		"data_dir":         c.DataDir.TFType(),
		"retry_join":       c.RetryJoin.TFType(),
		"server":           c.Server.TFType(),
		"bootstrap_expect": c.BootstrapExpect.TFType(),
		"log_file":         c.LogFile.TFType(),
		"log_level":        c.LogLevel.TFType(),
	}
}

func (c *consulConfig) optionalAttrs() map[string]struct{} {
	return map[string]struct{}{
		"bind_addr": {},
	}
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

func (c *consulConfig) Terraform5Value() tftypes.Value {
	typ := tftypes.Object{
		AttributeTypes: c.attrs(),
	}

	return tftypes.NewValue(typ, map[string]tftypes.Value{
		"bind_addr":        c.BindAddr.TFValue(),
		"data_dir":         c.DataDir.TFValue(),
		"datacenter":       c.Datacenter.TFValue(),
		"retry_join":       c.RetryJoin.TFValue(),
		"server":           c.Server.TFValue(),
		"bootstrap_expect": c.BootstrapExpect.TFValue(),
		"log_file":         c.LogFile.TFValue(),
		"log_level":        c.LogLevel.TFValue(),
	})
}

// FromTerraform5Value unmarshals the value to the struct.
func (c *consulConfig) FromTerraform5Value(val tftypes.Value) error {
	_, err := mapAttributesTo(val, map[string]interface{}{
		"bind_addr":        c.BindAddr,
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

// ToHCLConfig returns the consul config in the remoteflight HCLConfig format.
func (c *consulConfig) ToHCLConfig() *hcl.Builder {
	hlcBuilder := hcl.NewBuilder()

	if bindAddr, ok := c.BindAddr.Get(); ok {
		hlcBuilder.AppendAttribute("bind_addr", bindAddr)
	}

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

func (s *consulStartStateV1) startConsul(ctx context.Context, transport it.Transport) error {
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

	_, err = remoteflight.CreateOrUpdateUser(ctx, transport, remoteflight.NewUser(
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

	unit := systemd.Unit{
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
		err = remoteflight.CopyFile(ctx, transport, remoteflight.NewCopyFileRequest(
			remoteflight.WithCopyFileDestination(licensePath),
			remoteflight.WithCopyFileChmod("644"),
			remoteflight.WithCopyFileChown(fmt.Sprintf("%s:%s", consulUsername, consulUsername)),
			remoteflight.WithCopyFileContent(tfile.NewReader(license)),
		))
		if err != nil {
			return fmt.Errorf("failed to copy consul license, due to: %w", err)
		}

		// Validate the Consul license file
		err = consul.ValidateConsulLicense(ctx, transport, consul.NewValidateFileRequest(
			consul.WithValidateConfigBinPath(s.BinPath.Value()),
			consul.WithValidateFilePath(licensePath),
		))
		if err != nil {
			return fmt.Errorf("consul license validation failed, due to: %w", err)
		}

		unit["Service"]["Environment"] = "CONSUL_LICENSE_PATH=" + licensePath
	}

	sysd := systemd.NewClient(transport, log.NewLogger(ctx))

	// Write the systemd unit
	err = sysd.CreateUnitFile(ctx, systemd.NewCreateUnitFileRequest(
		systemd.WithUnitUnitPath(fmt.Sprintf("/etc/systemd/system/%s.service", unitName)),
		systemd.WithUnitChmod("644"),
		systemd.WithUnitChown(fmt.Sprintf("%s:%s", consulUsername, consulUsername)),
		systemd.WithUnitFile(unit),
	))
	if err != nil {
		return fmt.Errorf("failed to create the consul systemd unit, due to: %w", err)
	}

	_, err = sysd.RunSystemctlCommand(ctx, systemd.NewRunSystemctlCommand(
		systemd.WithSystemctlCommandSubCommand(systemd.SystemctlSubCommandDaemonReload),
	))
	if err != nil {
		return fmt.Errorf("failed to daemon-reload systemd after writing the consul systemd unit, due to: %w", err)
	}

	config := s.Config.ToHCLConfig()

	// Create the consul HCL configuration file
	err = hcl.CreateHCLConfigFile(ctx, transport, hcl.NewCreateHCLConfigFileRequest(
		hcl.WithHCLConfigFilePath(configFilePath),
		hcl.WithHCLConfigChmod("644"),
		hcl.WithHCLConfigChown(fmt.Sprintf("%s:%s", consulUsername, consulUsername)),
		hcl.WithHCLConfigFile(config),
	))
	if err != nil {
		return fmt.Errorf("failed to create the consul configuration file, due to: %w", err)
	}

	// Create the consul data directory
	err = remoteflight.CreateDirectory(ctx, transport, remoteflight.NewCreateDirectoryRequest(
		remoteflight.WithDirName(dataDir),
		remoteflight.WithDirChown(consulUsername),
	))
	if err != nil {
		return fmt.Errorf("failed to change ownership on data directory, due to: %w", err)
	}

	// Create the consul config directory
	err = remoteflight.CreateDirectory(ctx, transport, remoteflight.NewCreateDirectoryRequest(
		remoteflight.WithDirName(configDir),
		remoteflight.WithDirChown(consulUsername),
	))
	if err != nil {
		return fmt.Errorf("failed to change ownership on config directory, due to: %w", err)
	}

	// Validate the Consul config file
	err = consul.ValidateConsulConfig(ctx, transport, consul.NewValidateFileRequest(
		consul.WithValidateUsername(consulUsername),
		consul.WithValidateConfigBinPath(s.BinPath.Value()),
		consul.WithValidateFilePath(configFilePath),
	))
	if err != nil {
		return fmt.Errorf("failed to validate consul configuration file, due to: %w", err)
	}

	// A reasonable amount of time for all cluster nodes to come online, discover
	// each other and elect a leader.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Restart the service and wait for until systemd thinks that the process
	// is active and running.
	err = sysd.RestartService(ctx, "consul")
	if err != nil {
		return fmt.Errorf("failed to start the consul service, due to: %w", err)
	}

	// Wait for the consul cluster to be ready to service requests
	checks := []consul.CheckStater{
		consul.CheckStateHasSystemdEnabledAndRunningProperties(),
		consul.CheckStateNodeIsHealthy(),
		consul.CheckStateClusterHasLeader(),
	}

	if minNodes, ok := s.Config.BootstrapExpect.Get(); ok {
		checks = append(checks,
			consul.CheckStateClusterHasMinNVoters(uint(minNodes)),
			consul.CheckStateClusterHasMinNHealthyNodes(uint(minNodes)),
		)
	}

	state, err := consul.WaitForState(ctx, transport, consul.NewStateRequest(
		consul.WithStateRequestFlightControlUseHomeDir(),
		consul.WithStateRequestSystemdUnitName(unitName),
	), checks...)
	if err != nil {
		err = fmt.Errorf("failed to start the consul service: %w", err)
		if state != nil {
			err = fmt.Errorf(
				"%w\nConsul State after starting the consul systemd service:\n%s",
				err, istrings.Indent("  ", state.String()),
			)
		}
	}

	return err
}
