// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/diags"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/log"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight/hcl"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight/systemd"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight/vault"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/state"
	istrings "github.com/hashicorp-forge/terraform-provider-enos/internal/strings"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight"
	resource "github.com/hashicorp-forge/terraform-provider-enos/internal/server/resourcerouter"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	tfile "github.com/hashicorp-forge/terraform-provider-enos/internal/transport/file"
)

const (
	defaultRaftDataDir     = "/opt/raft/data"
	raftStorageType        = "raft"
	defaultVaultConfigMode = "file"
)

type vaultStart struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resource.Resource = (*vaultStart)(nil)

type vaultStartStateV1 struct {
	ID              *tfString
	BinPath         *tfString
	Config          *vaultConfig
	ConfigDir       *tfString
	ConfigMode      *tfString
	License         *tfString
	Status          *tfNum
	SystemdUnitName *tfString
	ManageService   *tfBool
	Transport       *embeddedTransportV1
	Username        *tfString
	Environment     *tfStringMap

	failureHandlers
}

type vaultConfig struct {
	ClusterName *tfString
	APIAddr     *tfString
	ClusterAddr *tfString
	Listener    *vaultListenerConfig
	LogLevel    *tfString
	Storage     *vaultStorageConfig
	Seal        *vaultConfigBlock // Single seal configuration
	Seals       *vaultSealsConfig // HA Seal configuration
	Telemetry   *dynamicPseudoTypeBlock
	UI          *tfBool
}

var _ state.State = (*vaultStartStateV1)(nil)

func newVaultStart() *vaultStart {
	return &vaultStart{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newVaultStartStateV1() *vaultStartStateV1 {
	transport := newEmbeddedTransport()
	fh := failureHandlers{
		TransportDebugFailureHandler(transport),
		GetApplicationLogsFailureHandler(transport, []string{"vault"}),
	}

	return &vaultStartStateV1{
		ID:              newTfString(),
		BinPath:         newTfString(),
		Config:          newVaultConfig(),
		ConfigDir:       newTfString(),
		ConfigMode:      newTfString(),
		License:         newTfString(),
		Status:          newTfNum(),
		SystemdUnitName: newTfString(),
		ManageService:   newTfBool(),
		Transport:       transport,
		Username:        newTfString(),
		Environment:     newTfStringMap(),
		failureHandlers: fh,
	}
}

func newVaultConfig() *vaultConfig {
	return &vaultConfig{
		ClusterName: newTfString(),
		APIAddr:     newTfString(),
		ClusterAddr: newTfString(),
		LogLevel:    newTfString(),
		Listener:    newVaultListenerConfig(),
		Seal:        newVaultConfigBlock("config", "seal"),
		Seals:       newVaultSealsConfig(),
		Storage:     newVaultStorageConfig(),
		Telemetry:   newDynamicPseudoTypeBlock(),
		UI:          newTfBool(),
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
//  1. Unmarshal the RawState to the corresponding tftypes.Value that matches
//     schema version of the state we're upgrading from.
//  2. Create a new tftypes.Value for the current state and migrate the old
//     values to the new values.
//  3. Upgrade the existing state with the new values and return the marshaled
//     version of the current upgraded state.
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
func (r *vaultStart) PlanResourceChange(ctx context.Context, req resource.PlanResourceChangeRequest, res *resource.PlanResourceChangeResponse) {
	priorState := newVaultStartStateV1()
	proposedState := newVaultStartStateV1()
	res.PlannedState = proposedState

	transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, r, req, res)
	if diags.HasErrors(res.Diagnostics) {
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
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *vaultStart) ApplyResourceChange(ctx context.Context, req resource.ApplyResourceChangeRequest, res *resource.ApplyResourceChangeResponse) {
	priorState := newVaultStartStateV1()
	plannedState := newVaultStartStateV1()
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

	plannedState.ID.Set("static")

	client, err := transport.Client(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Transport Error", err))
		return
	}
	defer client.Close()

	// If our priorState ID is blank then we're creating the resource
	if _, ok := priorState.ID.Get(); !ok {
		err = plannedState.startVault(ctx, client)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Vault Start Error", err))
			return
		}
	} else if reflect.DeepEqual(plannedState, priorState) {
		err = plannedState.startVault(ctx, client)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Vault Start Error", err))
			return
		}
	}
}

// Schema is the file states Terraform schema.
func (s *vaultStartStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			DescriptionKind: tfprotov6.StringKindMarkdown,
			Description: docCaretToBacktick(`
The ^enos_vault_start^ resource is capable of configuring and starting a Vault
service. It handles creating the configuration directory, the configuration file,
the license file, the systemd unit, and starting the service.

*NOTE: Until recently we were not able to implement optional attributes for the config attribute.
As such, you will need to provide _all_ values except for ^seals^ until we make all config optional.*
`),
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:        "id",
					Type:        tftypes.String,
					Computed:    true,
					Description: resourceStaticIDDescription,
				},
				{
					Name:        "bin_path", // where the vault binary is
					Type:        tftypes.String,
					Required:    true,
					Description: "The fully qualified path to the vault binary",
				},
				{
					Name:            "config",
					Type:            s.Config.Terraform5Type(),
					Required:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description: docCaretToBacktick(`
The vault configuration
- ^config.api_addr^ (String) The Vault [api_addr](https://developer.hashicorp.com/vault/docs/configuration#api_addr) value
- ^config.cluster_addr^ (String) The Vault [cluster_addr](https://developer.hashicorp.com/vault/docs/configuration#cluster_addr) value
- ^config.cluster_name^ (String) The Vault [cluster_addr](https://developer.hashicorp.com/vault/docs/configuration#cluster_addr) value
- ^config.listener^ (Object) The Vault [listener](https://developer.hashicorp.com/vault/docs/configuration/listener) stanza
- ^config.listener.type^ (String) The Vault [listener](https://developer.hashicorp.com/vault/docs/configuration/listener/tcp) stanza value. Currently 'tcp' is the only supported listener
- ^config.listener.attributes^ (Object) The Vault [listener](https://developer.hashicorp.com/vault/docs/configuration/listener/tcp#tcp-listener-parameters) top-level parameters for the listener
- ^config.listener.telemetry^ (Object) The Vault listener [telemetry](https://developer.hashicorp.com/vault/docs/configuration/listener/tcp#telemetry-parameters) stanza
- ^config.listener.profiling^ (Object) The Vault listener [profiling](https://developer.hashicorp.com/vault/docs/configuration/listener/tcp#profiling-parameters) stanza
- ^config.listener.inflight_requests_logging^ (Object) The Vault listener [inflight_requests_logging](https://developer.hashicorp.com/vault/docs/configuration/listener/tcp#inflight_requests_logging-parameters) stanza
- ^config.listener.custom_response_headers^ (Object) The Vault listener [custom_response_headers](https://developer.hashicorp.com/vault/docs/configuration/listener/tcp#custom_response_headers-parameters) stanza
- ^config.log_level^ (String) The Vault [log_level](https://developer.hashicorp.com/vault/docs/configuration#log_level)
- ^config.storage^ (Object, Optional) The Vault [storage](https://developer.hashicorp.com/vault/docs/configuration/storage) stanza
- ^config.storage.type^ (String) The Vault [storage](https://developer.hashicorp.com/vault/docs/configuration/storage) type
- ^config.storage.attributes^ (Object) The Vault [storage](https://developer.hashicorp.com/vault/docs/configuration/storage) parameters for the given storage type
- ^config.storage.retry_join^ (Object) The Vault integrated storage [retry_join](https://developer.hashicorp.com/vault/docs/configuration/storage/raft#retry_join-stanza) stanza
- ^config.seal^ (Object, Optional) The Vault [seal](https://developer.hashicorp.com/vault/docs/configuration/seal) stanza
- ^config.seal.type^ (String) The Vault [seal](https://developer.hashicorp.com/vault/docs/configuration/seal) type
- ^config.seal.attributes^ (String) The Vault [seal](https://developer.hashicorp.com/vault/docs/configuration/seal) parameters for the given seal type
- ^config.seals^ (Object, Optional) Vault Enterprise [HA seal](https://developer.hashicorp.com/vault/docs/configuration/seal/seal-ha) configuration. Cannot be used in conjunction with ^config.seal^. Up to three seals can be defined but only one is required.
- ^config.seals.primary^ (Object) The primary [HA seal](https://developer.hashicorp.com/vault/docs/configuration/seal/seal-ha) stanza. Primary has priority 1
- ^config.seals.primary.type^ (String) The Vault [seal](https://developer.hashicorp.com/vault/docs/configuration/seal) type
- ^config.seals.primary.attributes^ (String) The Vault [seal](https://developer.hashicorp.com/vault/docs/configuration/seal) parameters for the given seal type
- ^config.seals.secondary^ (Object) The secondary [HA seal](https://developer.hashicorp.com/vault/docs/configuration/seal/seal-ha) stanza. Secondary has priority 2
- ^config.seals.secondary.type^ (String) The Vault [seal](https://developer.hashicorp.com/vault/docs/configuration/seal) type
- ^config.seals.secondary.attributes^ (String) The Vault [seal](https://developer.hashicorp.com/vault/docs/configuration/seal) parameters for the given seal type
- ^config.seals.tertiary^ (Object) The tertiary [HA seal](https://developer.hashicorp.com/vault/docs/configuration/seal/seal-ha) stanza. Tertiary has priority 3
- ^config.seals.tertiary.type^ (String) The Vault [seal](https://developer.hashicorp.com/vault/docs/configuration/seal) type
- ^config.seals.tertiary.attributes^ (String) The Vault [seal](https://developer.hashicorp.com/vault/docs/configuration/seal) parameters for the given seal type
- ^config.telemetry^ (Object, Optional) The Vault [telemetry](https://developer.hashicorp.com/vault/docs/configuration/telemetry#telemetry-parameters) stanza
`),
				},
				{
					Name:        "config_dir", // where to write vault config
					Type:        tftypes.String,
					Optional:    true,
					Description: "The path where Vault configuration will reside",
				},
				{
					Name:        "config_mode", // preferred method of configuring vault, file or env
					Type:        tftypes.String,
					Optional:    true,
					Description: "The preferred method of configuring vault. Valid options are 'file' or 'env'",
				},
				{
					Name:        "license", // the vault license
					Type:        tftypes.String,
					Optional:    true,
					Sensitive:   true,
					Description: "The Vault Enterprise license",
				},
				{
					Name:            "status", // the vault status code
					Type:            tftypes.Number,
					Computed:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description: `
The Vault status code returned when starting the service.

|code|meaning|
|0|Initialized, Unsealed|
|1|Error|
|2|Sealed|
|9|Unknown, we couldn't get the status from Vault|
`,
				},
				{
					Name:        "unit_name", // sysmted unit name
					Type:        tftypes.String,
					Optional:    true,
					Description: "The systemd unit name",
				},
				{
					Name:        "manage_service",
					Type:        tftypes.Bool,
					Optional:    true,
					Description: "Whether or not Enos will be responsible for creating and managing the systemd unit for Vault",
				},
				{
					Name:        "username", // vault username
					Type:        tftypes.String,
					Optional:    true,
					Description: "The local service user name",
				},
				{
					Name:        "environment",
					Description: "An optional map of key/value pairs for additional environment variables to set when running the vault service.",
					Type:        tftypes.Map{ElementType: tftypes.String},
					Optional:    true,
				},
				s.Transport.SchemaAttributeTransport(supportsSSH),
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

	if err := checkK8STransportNotConfigured(s, "enos_vault_start"); err != nil {
		return err
	}

	if _, ok := s.BinPath.Get(); !ok {
		return ValidationError("you must provide a vault binary path", "bin_path")
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *vaultStartStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]any{
		"bin_path":       s.BinPath,
		"config_dir":     s.ConfigDir,
		"config_mode":    s.ConfigMode,
		"id":             s.ID,
		"license":        s.License,
		"status":         s.Status,
		"unit_name":      s.SystemdUnitName,
		"manage_service": s.ManageService,
		"username":       s.Username,
		"environment":    s.Environment,
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
		"config_mode":    s.ConfigMode.TFType(),
		"id":             s.ID.TFType(),
		"license":        s.License.TFType(),
		"status":         s.Status.TFType(),
		"unit_name":      s.SystemdUnitName.TFType(),
		"manage_service": s.ManageService.TFType(),
		"transport":      s.Transport.Terraform5Type(),
		"username":       s.Username.TFType(),
		"environment":    s.Environment.TFType(),
	}}
}

// Terraform5Value is the file state tftypes.Value.
func (s *vaultStartStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"bin_path":       s.BinPath.TFValue(),
		"config":         s.Config.Terraform5Value(),
		"config_dir":     s.ConfigDir.TFValue(),
		"config_mode":    s.ConfigMode.TFValue(),
		"id":             s.ID.TFValue(),
		"license":        s.License.TFValue(),
		"status":         s.Status.TFValue(),
		"unit_name":      s.SystemdUnitName.TFValue(),
		"manage_service": s.ManageService.TFValue(),
		"transport":      s.Transport.Terraform5Value(),
		"username":       s.Username.TFValue(),
		"environment":    s.Environment.TFValue(),
	})
}

// EmbeddedTransport returns a pointer the resources embedded transport.
func (s *vaultStartStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}

func (c *vaultConfig) Terraform5Type() tftypes.Type {
	return tftypes.Object{
		AttributeTypes:     c.attrs(),
		OptionalAttributes: c.optionalAttrs(),
	}
}

func (c *vaultConfig) attrs() map[string]tftypes.Type {
	return map[string]tftypes.Type{
		"api_addr":     tftypes.String,
		"cluster_addr": tftypes.String,
		"cluster_name": c.ClusterName.TFType(),
		"listener":     c.Listener.Terraform5Type(),
		"log_level":    tftypes.String,
		"storage":      c.Storage.Terraform5Type(),
		"seal":         c.Seal.Terraform5Type(),
		"seals":        c.Seals.Terraform5Type(),
		"telemetry":    c.Telemetry.TFType(),
		"ui":           c.UI.TFType(),
	}
}

func (c *vaultConfig) optionalAttrs() map[string]struct{} {
	return map[string]struct{}{
		"seal":      {},
		"seals":     {},
		"storage":   {},
		"telemetry": {},
	}
}

func (c *vaultConfig) Terraform5Value() tftypes.Value {
	typ := tftypes.Object{
		AttributeTypes: c.attrs(),
	}

	telemetry, err := c.Telemetry.TFValue()
	if err != nil {
		panic(err)
	}

	return tftypes.NewValue(typ, map[string]tftypes.Value{
		"cluster_name": c.ClusterName.TFValue(),
		"api_addr":     c.APIAddr.TFValue(),
		"cluster_addr": c.ClusterAddr.TFValue(),
		"listener":     c.Listener.Terraform5Value(),
		"log_level":    c.LogLevel.TFValue(),
		"seal":         c.Seal.Terraform5Value(),
		"seals":        c.Seals.Terraform5Value(),
		"storage":      c.Storage.Terraform5Value(),
		"telemetry":    telemetry,
		"ui":           c.UI.TFValue(),
	})
}

// FromTerraform5Value unmarshals the value to the struct.
func (c *vaultConfig) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]any{
		"api_addr":     c.APIAddr,
		"cluster_addr": c.ClusterAddr,
		"cluster_name": c.ClusterName,
		"log_level":    c.LogLevel,
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

	seals, ok := vals["seals"]
	if ok {
		err = c.Seals.FromTerraform5Value(seals)
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

	telemetry, ok := vals["telemetry"]
	if ok {
		err = c.Telemetry.FromTFValue(telemetry)
		if err != nil {
			return err
		}
	}

	return nil
}

// Render takes a preferred configuration mode (file, env) and return an HCL builder, a map of
// environment variables, and any errors that are encountered along the way. As not all configuration
// is settable via environment variables we'll often have to use both the HCL and env variables
// regardless of the preferred mode. We support two configuration modes to allow us to test
// both env var and config code paths.
//
//nolint:gocyclo,cyclop
func (c *vaultConfig) Render(configMode string) (*hcl.Builder, map[string]string, error) {
	if configMode != "env" && configMode != "file" {
		return nil, nil, fmt.Errorf("unsupported config_mode %s, expected 'env' or 'file'", configMode)
	}

	hclBuilder := hcl.NewBuilder()
	envVars := map[string]string{}

	if apiAddr, ok := c.APIAddr.Get(); ok {
		if configMode == "file" {
			hclBuilder.AppendAttribute("api_addr", apiAddr)
		} else {
			envVars["VAULT_API_ADDR"] = apiAddr
		}
	}

	if clusterAddr, ok := c.ClusterAddr.Get(); ok {
		if configMode == "file" {
			hclBuilder.AppendAttribute("cluster_addr", clusterAddr)
		} else {
			envVars["VAULT_CLUSTER_ADDR"] = clusterAddr
		}
	}

	if ui, ok := c.UI.Get(); ok {
		if configMode == "file" {
			hclBuilder.AppendAttribute("ui", ui)
		} else {
			envVars["VAULT_UI"] = strconv.FormatBool(ui)
		}
	}

	if ll, ok := c.LogLevel.Get(); ok {
		if configMode == "file" {
			hclBuilder.AppendAttribute("log_level", ll)
		} else {
			envVars["VAULT_LOG_LEVEL"] = ll
		}
	}

	if label, ok := c.Listener.Type.Get(); ok {
		listenerBlock := hclBuilder.AppendBlock("listener", []string{label})
		if attrs, ok := c.Listener.Attrs.Object.GetObject(); ok {
			listenerBlock.AppendAttributes(attrs)
		}
		if telemetry, ok := c.Listener.Telemetry.Object.GetObject(); ok {
			listenerBlock.AppendBlock("telemetry", nil).AppendAttributes(telemetry)
		}
		if profiling, ok := c.Listener.Profiling.Object.GetObject(); ok {
			listenerBlock.AppendBlock("profiling", nil).AppendAttributes(profiling)
		}
		if inflight_requests_logging, ok := c.Listener.IRL.Object.GetObject(); ok {
			listenerBlock.AppendBlock("inflight_requests_logging", nil).AppendAttributes(inflight_requests_logging)
		}
		if custom_response_headers, ok := c.Listener.CRH.Object.GetObject(); ok {
			listenerBlock.AppendBlock("custom_response_headers", nil).AppendAttributes(custom_response_headers)
		}
	}

	// Ignore shamir seals because they don't actually have a config stanza
	if label, ok := c.Seal.Type.Get(); ok && label != "shamir" {
		if attrs, ok := c.Seal.Attrs.GetObject(); ok {
			if configMode == "file" {
				hclBuilder.AppendBlock("seal", []string{label}).AppendAttributes(attrs)
			} else {
				trans := sealAttrEnvVarTranslator{}
				sealEnvVars, err := trans.ToEnvVars(label, attrs)
				if err != nil {
					return nil, nil, err
				}
				maps.Copy(envVars, sealEnvVars)
			}
		}
	}

	if c.Seals.needsMultiseal() {
		hclBuilder.AppendAttribute("enable_multiseal", true)
	}
	for priority, seal := range c.Seals.Value() {
		if label, ok := seal.Type.Get(); ok && label != "shamir" && label != "none" {
			if attrs, ok := seal.Attrs.GetObject(); ok {
				switch priority {
				case "primary":
					if _, ok := attrs["priority"]; !ok {
						attrs["priority"] = "1"
					}
					if _, ok := attrs["name"]; !ok {
						attrs["name"] = "primary"
					}
				case "secondary":
					if _, ok := attrs["priority"]; !ok {
						attrs["priority"] = "2"
					}
					if _, ok := attrs["name"]; !ok {
						attrs["name"] = "secondary"
					}
				case "tertiary":
					if _, ok := attrs["priority"]; !ok {
						attrs["priority"] = "3"
					}
					if _, ok := attrs["name"]; !ok {
						attrs["name"] = "tertiary"
					}
				default:
				}

				// Only write our seal config as env variables if we've only been configured with one
				if !c.Seals.needsMultiseal() && configMode == "env" {
					trans := sealAttrEnvVarTranslator{}
					sealEnvVars, err := trans.ToEnvVars(label, attrs)
					if err != nil {
						return nil, nil, err
					}
					maps.Copy(envVars, sealEnvVars)
				} else {
					hclBuilder.AppendBlock("seal", []string{label}).AppendAttributes(attrs)
				}
			}
		}
	}

	if c.Storage != nil && c.Storage.Type != nil {
		if storageLabel, ok := c.Storage.Type.Get(); ok {
			storageBlock := hclBuilder.AppendBlock("storage", []string{storageLabel})
			attrs, ok := c.Storage.Attrs.Object.GetObject()
			if ok { // Add our attributes to the storage block
				storageBlock.AppendAttributes(attrs)

				if storageLabel == raftStorageType {
					// Handle integrated storage defaults. We do this for backwards compatibility with older
					// provider versions. Make sure to only set defaults if the user has not passed in the
					// keys.
					if _, ok := attrs["path"]; !ok {
						storageBlock.AppendAttribute("path", defaultRaftDataDir)
					}

					retryJoinBlock := storageBlock.AppendBlock("retry_join", []string{})
					retryJoinAttrs, ok := c.Storage.RetryJoin.Object.GetObject()
					if ok {
						// We've been configured with retry_join so we'll set the attrs
						retryJoinBlock.AppendAttributes(retryJoinAttrs)
					} else {
						// The user has not configured retry_join set so we'll use the old defaults for
						// backwards compat.
						clusterName, ok := c.ClusterName.Get()
						if !ok {
							// This shouldn't ever happen...
							return nil, nil, errors.New("enos_vault_start.cluster_name must be set")
						}

						retryJoinBlock.AppendAttribute("auto_join", "provider=aws tag_key=Type tag_value="+clusterName).
							AppendAttribute("auto_join_scheme", "http")
					}
				}
			}
		}
	}

	if telemetry, ok := c.Telemetry.Object.GetObject(); ok {
		hclBuilder.AppendBlock("telemetry", nil).AppendAttributes(telemetry)
	}

	return hclBuilder, envVars, nil
}

func (s *vaultStartStateV1) startVault(ctx context.Context, transport it.Transport) error {
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

	if _, ok := s.Config.LogLevel.Get(); !ok {
		s.Config.LogLevel.Set("info")
	}

	envFilePath := "/etc/vault.d/vault.env"

	envVars := map[string]string{}
	if environment, ok := s.Environment.Get(); ok {
		for key, value := range environment {
			if val, valOk := value.Get(); valOk {
				envVars[key] = val
			}
		}
	}

	_, err = remoteflight.CreateOrUpdateUser(ctx, transport, remoteflight.NewUser(
		remoteflight.WithUserName(vaultUsername),
		remoteflight.WithUserHomeDir(configDir),
		remoteflight.WithUserShell("/bin/false"),
	))
	if err != nil {
		return fmt.Errorf("failed to find or create the vault user, due to: %w", err)
	}

	// Copy the license file if we have one
	if license, ok := s.License.Get(); ok {
		err = remoteflight.CopyFile(ctx, transport, remoteflight.NewCopyFileRequest(
			remoteflight.WithCopyFileDestination(licensePath),
			remoteflight.WithCopyFileChmod("640"),
			remoteflight.WithCopyFileChown(fmt.Sprintf("%s:%s", vaultUsername, vaultUsername)),
			remoteflight.WithCopyFileContent(tfile.NewReader(license)),
		))
		if err != nil {
			return fmt.Errorf("failed to copy vault license, due to: %w", err)
		}

		envVars["VAULT_LICENSE_PATH"] = licensePath
	}

	// Render our vault configuration into HCL and/or environment variables.
	configMode, ok := s.ConfigMode.Get()
	if configMode == "" || !ok {
		configMode = defaultVaultConfigMode
	}

	hclConfig, configEnv, err := s.Config.Render(configMode)
	if err != nil {
		return fmt.Errorf("failed to create the vault HCL configuration, due to: %w", err)
	}

	maps.Copy(envVars, configEnv)

	// Write our config file
	err = hcl.CreateHCLConfigFile(ctx, transport, hcl.NewCreateHCLConfigFileRequest(
		hcl.WithHCLConfigFilePath(configFilePath),
		hcl.WithHCLConfigChmod("640"),
		hcl.WithHCLConfigChown(fmt.Sprintf("%s:%s", vaultUsername, vaultUsername)),
		hcl.WithHCLConfigFile(hclConfig),
	))
	if err != nil {
		return fmt.Errorf("failed to create the vault configuration file, due to: %w", err)
	}

	// Write our environment variables file.
	envVarsString := strings.Builder{}
	for k, v := range envVars {
		envVarsString.WriteString(k + "=" + v + "\n")
	}

	err = remoteflight.CopyFile(ctx, transport, remoteflight.NewCopyFileRequest(
		remoteflight.WithCopyFileDestination(envFilePath),
		remoteflight.WithCopyFileChmod("644"),
		remoteflight.WithCopyFileChown(fmt.Sprintf("%s:%s", vaultUsername, vaultUsername)),
		remoteflight.WithCopyFileContent(tfile.NewReader(envVarsString.String())),
	))
	if err != nil {
		return fmt.Errorf("failed to create the vault environment file, due to: %w", err)
	}

	sysd := systemd.NewClient(transport, log.NewLogger(ctx))
	unitName := "vault"
	if unit, ok := s.SystemdUnitName.Get(); ok {
		unitName = unit
	}

	// Manage the vault systemd service ourselves unless it has explicitly been
	// set that we should not.
	if manage, set := s.ManageService.Get(); !set || (set && manage) {
		unit := systemd.Unit{
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
				"Type":                  "notify",
				"EnvironmentFile":       envFilePath,
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
			systemd.WithUnitChmod("644"),
			systemd.WithUnitChown(fmt.Sprintf("%s:%s", vaultUsername, vaultUsername)),
			systemd.WithUnitFile(unit),
		))
		if err != nil {
			return fmt.Errorf("failed to create the vault systemd unit, due to: %w", err)
		}

		_, err = sysd.RunSystemctlCommand(ctx, systemd.NewRunSystemctlCommand(
			systemd.WithSystemctlCommandSubCommand(systemd.SystemctlSubCommandDaemonReload),
		))
		if err != nil {
			return fmt.Errorf("failed to daemon-reload systemd after writing the vault systemd unit, due to: %w", err)
		}
	}

	if storageType, ok := s.Config.Storage.Type.Get(); ok && storageType == raftStorageType {
		err = remoteflight.CreateDirectory(ctx, transport, remoteflight.NewCreateDirectoryRequest(
			remoteflight.WithDirName(defaultRaftDataDir),
			remoteflight.WithDirChown(vaultUsername),
		))
		if err != nil {
			return fmt.Errorf("failed to change ownership on raft data directory, due to: %w", err)
		}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	// Restart the service and wait for it to be running
	err = sysd.RestartService(timeoutCtx, unitName)
	if err != nil {
		return fmt.Errorf("failed to start the vault service, due to: %w", err)
	}

	state, err := vault.WaitForState(timeoutCtx, transport, vault.NewStateRequest(
		vault.WithStateRequestFlightControlUseHomeDir(),
		vault.WithStateRequestBinPath(s.BinPath.Value()),
		vault.WithStateRequestVaultAddr(s.Config.APIAddr.Value()),
		vault.WithStateRequestSystemdUnitName(unitName),
	), vault.CheckStateHasSystemdEnabledAndRunningProperties(),
		vault.CheckStateSealStateIsKnown(),
	)

	statusCode := vault.StatusUnknown
	if state != nil {
		var err1 error
		statusCode, err1 = state.StatusCode()
		err = errors.Join(err, err1)
	}

	s.Status.Set(int(statusCode))

	if err != nil {
		err = fmt.Errorf("failed to start the vault service: %w", err)
		if state != nil {
			err = fmt.Errorf(
				"%w\nCluster State after starting the vault systemd service:\n%s",
				err, istrings.Indent("  ", state.String()),
			)
		}
	}

	return err
}

type sealAttrEnvVarTranslator struct{}

func (s *sealAttrEnvVarTranslator) sealAttrToString(attr any) (string, error) {
	switch t := attr.(type) {
	case string:
		s, ok := attr.(string)
		if !ok {
			return "", fmt.Errorf("could not cast attr of type %v to string", t)
		}

		return s, nil
	case int:
		return strconv.FormatInt(int64(attr.(int)), 10), nil
	default:
		return "", fmt.Errorf("unsupported seal attribute value, got %v for %s, must be string or integer", t, attr)
	}
}

// ToEnvVars takes our seal attributes and coverts them into the corresponding vault
// environment variable key/value pairs.
func (s *sealAttrEnvVarTranslator) ToEnvVars(seal string, in map[string]any) (map[string]string, error) {
	if seal == "" {
		return nil, errors.New("no seal type provided")
	}

	if len(in) < 1 {
		return nil, errors.New("no seal attributes provided")
	}

	switch seal {
	case "alicloudkms":
		return s.translateAliCloudKMS(in)
	case "awskms":
		return s.translateAWSKMS(in)
	case "azurekeyvault":
		return s.translateAzureKeyVault(in)
	case "gcpckms":
		return s.translateGCPKMS(in)
	case "ocikms":
		return s.translateOCIKMS(in)
	case "pkcs11":
		return s.translatePKCS11(in)
	case "transit":
		return s.translateTransit(in)
	default:
		return nil, fmt.Errorf("%v is not a supported seal type", seal)
	}
}

func (s *sealAttrEnvVarTranslator) translateAliCloudKMS(in map[string]any) (map[string]string, error) {
	out := map[string]string{"VAULT_SEAL_TYPE": "alicloudkms"}

	for k, v := range in {
		v, err := s.sealAttrToString(v)
		if err != nil {
			return nil, err
		}
		switch k {
		case "kms_key_id":
			out["VAULT_ALICLOUDKMS_SEAL_KEY_ID"] = v
		case "name", "priority":
		default:
			out["ALICLOUD_"+strings.ToUpper(k)] = v
		}
	}

	return out, nil
}

func (s *sealAttrEnvVarTranslator) translateAWSKMS(in map[string]any) (map[string]string, error) {
	out := map[string]string{"VAULT_SEAL_TYPE": "awskms"}

	for k, v := range in {
		v, err := s.sealAttrToString(v)
		if err != nil {
			return nil, err
		}
		switch k {
		case "kms_key_id":
			out["VAULT_AWSKMS_SEAL_KEY_ID"] = v
		case "secret_key":
			out["AWS_SECRET_ACCESS_KEY"] = v
		case "name", "priority":
		default:
			out["AWS_"+strings.ToUpper(k)] = v
		}
	}

	return out, nil
}

func (s *sealAttrEnvVarTranslator) translateAzureKeyVault(in map[string]any) (map[string]string, error) {
	out := map[string]string{"VAULT_SEAL_TYPE": "azurekeyvault"}

	for k, v := range in {
		v, err := s.sealAttrToString(v)
		if err != nil {
			return nil, err
		}
		switch k {
		case "vault_name", "key_name":
			out["VAULT_AZUREKEYVAULT_"+strings.ToUpper(k)] = v
		case "resource":
			out["AZURE_AD_RESOURCE"] = v
		case "name", "priority":
		default:
			out["AZURE_"+strings.ToUpper(k)] = v
		}
	}

	return out, nil
}

func (s *sealAttrEnvVarTranslator) translateGCPKMS(in map[string]any) (map[string]string, error) {
	out := map[string]string{"VAULT_SEAL_TYPE": "gcpckms"}

	for k, v := range in {
		v, err := s.sealAttrToString(v)
		if err != nil {
			return nil, err
		}
		switch k {
		case "key_ring", "crypto_ring", "crypto_key":
			out["VAULT_GCPCKMS_SEAL_"+strings.ToUpper(k)] = v
		case "name", "priority":
		default:
			out["GOOGLE_"+strings.ToUpper(k)] = v
		}
	}

	return out, nil
}

func (s *sealAttrEnvVarTranslator) translatePKCS11(in map[string]any) (map[string]string, error) {
	out := map[string]string{"VAULT_SEAL_TYPE": "pkcs11"}

	for k, v := range in {
		v, err := s.sealAttrToString(v)
		if err != nil {
			return nil, err
		}
		switch k {
		case "default_hmac_key_label":
			out["VAULT_HSM_HMAC_DEFAULT_KEY_LABEL"] = v
		case "name", "priority":
		default:
			out["VAULT_HSM_"+strings.ToUpper(k)] = v
		}
	}

	return out, nil
}

func (s *sealAttrEnvVarTranslator) translateOCIKMS(in map[string]any) (map[string]string, error) {
	out := map[string]string{"VAULT_SEAL_TYPE": "ocikms"}

	for k, v := range in {
		v, err := s.sealAttrToString(v)
		if err != nil {
			return nil, err
		}
		switch k {
		case "key_id":
			out["VAULT_OCIKMS_SEAL_KEY_ID"] = v
		case "name", "priority":
		default:
			out["VAULT_OCIKMS_"+strings.ToUpper(k)] = v
		}
	}

	return out, nil
}

func (s *sealAttrEnvVarTranslator) translateTransit(in map[string]any) (map[string]string, error) {
	out := map[string]string{"VAULT_SEAL_TYPE": "transit"}

	for k, v := range in {
		v, err := s.sealAttrToString(v)
		if err != nil {
			return nil, err
		}
		switch k {
		case "address":
			out["VAULT_ADDR"] = v
		case "token", "namespace":
			out["VAULT_"+strings.ToUpper(k)] = v
		case "key_name", "mount_path", "disable_renewal":
			out["VAULT_TRANSIT_SEAL_"+strings.ToUpper(k)] = v
		case "tls_ca_cert", "tls_client_cert", "tls_client_key", "tls_skip_verify":
			out["VAULT_"+strings.ToUpper(strings.TrimPrefix(k, "tls_"))] = v
		case "tls_server_name":
			out["VAULT_TLS_SERVER_NAME"] = v
		case "key_id_prefix", "name", "priority":
			// This doesn't have a documented env var equivalent
		default:
			return nil, fmt.Errorf("unknown transit seal key: %v", k)
		}
	}

	return out, nil
}
