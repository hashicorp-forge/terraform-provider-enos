// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/diags"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight/vault"
	resource "github.com/hashicorp-forge/terraform-provider-enos/internal/server/resourcerouter"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/state"
	istrings "github.com/hashicorp-forge/terraform-provider-enos/internal/strings"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
)

type vaultInit struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resource.Resource = (*vaultInit)(nil)

type vaultInitStateV1 struct {
	ID              *tfString
	BinPath         *tfString
	SystemdUnitName *tfString // when using systemd to manage service
	VaultAddr       *tfString
	Transport       *embeddedTransportV1
	// inputs
	KeyShares         *tfNum
	KeyThreshold      *tfNum
	PGPKeys           *tfStringSlice
	RecoveryShares    *tfNum
	RecoveryThreshold *tfNum
	RecoveryPGPKeys   *tfStringSlice
	RootTokenPGPKey   *tfString
	ConsulAuto        *tfBool
	ConsulService     *tfString
	StoredShares      *tfNum
	// outputs
	UnsealKeysB64         *tfStringSlice
	UnsealKeysHex         *tfStringSlice
	UnsealShares          *tfNum
	UnsealThreshold       *tfNum
	RecoveryKeysB64       *tfStringSlice
	RecoveryKeysHex       *tfStringSlice
	RecoveryKeysShares    *tfNum
	RecoveryKeysThreshold *tfNum
	RootToken             *tfString

	failureHandlers
}

var _ state.State = (*vaultInitStateV1)(nil)

func newVaultInit() *vaultInit {
	return &vaultInit{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newVaultInitStateV1() *vaultInitStateV1 {
	transport := newEmbeddedTransport()
	fh := failureHandlers{
		TransportDebugFailureHandler(transport),
		GetApplicationLogsFailureHandler(transport, []string{"vault"}),
	}

	return &vaultInitStateV1{
		ID:              newTfString(),
		BinPath:         newTfString(),
		VaultAddr:       newTfString(),
		SystemdUnitName: newTfString(),
		Transport:       transport,
		// inputs
		KeyShares:         newTfNum(),
		KeyThreshold:      newTfNum(),
		PGPKeys:           newTfStringSlice(),
		RecoveryShares:    newTfNum(),
		RecoveryThreshold: newTfNum(),
		RecoveryPGPKeys:   newTfStringSlice(),
		RootTokenPGPKey:   newTfString(),
		ConsulAuto:        newTfBool(),
		ConsulService:     newTfString(),
		StoredShares:      newTfNum(),
		// outputs
		UnsealKeysB64:         newTfStringSlice(),
		UnsealKeysHex:         newTfStringSlice(),
		UnsealShares:          newTfNum(),
		UnsealThreshold:       newTfNum(),
		RecoveryKeysB64:       newTfStringSlice(),
		RecoveryKeysHex:       newTfStringSlice(),
		RecoveryKeysShares:    newTfNum(),
		RecoveryKeysThreshold: newTfNum(),
		RootToken:             newTfString(),
		failureHandlers:       fh,
	}
}

func (r *vaultInit) Name() string {
	return "enos_vault_init"
}

func (r *vaultInit) Schema() *tfprotov6.Schema {
	return newVaultInitStateV1().Schema()
}

func (r *vaultInit) SetProviderConfig(meta tftypes.Value) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.FromTerraform5Value(meta)
}

func (r *vaultInit) GetProviderConfig() (*config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.Copy()
}

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (r *vaultInit) ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	newState := newVaultInitStateV1()

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
func (r *vaultInit) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	newState := newVaultInitStateV1()

	transportUtil.UpgradeResourceState(ctx, newState, req, res)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *vaultInit) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	newState := newVaultInitStateV1()

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
func (r *vaultInit) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	newState := newVaultInitStateV1()

	transportUtil.ImportResourceState(ctx, newState, req, res)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *vaultInit) PlanResourceChange(ctx context.Context, req resource.PlanResourceChangeRequest, res *resource.PlanResourceChangeResponse) {
	priorState := newVaultInitStateV1()
	proposedState := newVaultInitStateV1()
	res.PlannedState = proposedState

	transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, r, req, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	// When we're planning we need to determine if we've already applied before
	// or if we're planning to apply for the first time. If we already have an
	// ID we've been applied before and can simply plan to have the same state
	// since it'll be a no-op apply. If we haven't applied then we need to set
	// all of our computed outputs to unknown values.
	if _, ok := priorState.ID.Get(); !ok {
		proposedState.ID.Unknown = true
		proposedState.UnsealKeysB64.Unknown = true
		proposedState.UnsealKeysHex.Unknown = true
		proposedState.UnsealShares.Unknown = true
		proposedState.UnsealThreshold.Unknown = true
		proposedState.RecoveryKeysB64.Unknown = true
		proposedState.RecoveryKeysHex.Unknown = true
		proposedState.RecoveryKeysShares.Unknown = true
		proposedState.RecoveryKeysThreshold.Unknown = true
		proposedState.RootToken.Unknown = true
	}
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *vaultInit) ApplyResourceChange(ctx context.Context, req resource.ApplyResourceChangeRequest, res *resource.ApplyResourceChangeResponse) {
	priorState := newVaultInitStateV1()
	plannedState := newVaultInitStateV1()
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

	if !reflect.DeepEqual(priorState, plannedState) {
		err = plannedState.Init(ctx, client)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Vault Init Error", err))
			return
		}
	}
}

// Schema is the file states Terraform schema.
func (s *vaultInitStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			DescriptionKind: tfprotov6.StringKindMarkdown,
			Description: docCaretToBacktick(`
The ^enos_vault_init^ resource is capable initializing a Vault cluster.
`),
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:        "id",
					Type:        tftypes.String,
					Computed:    true,
					Description: resourceStaticIDDescription,
				},
				{
					Name:        "bin_path",
					Type:        tftypes.String,
					Required:    true,
					Description: "The fully qualified path to the vault binary",
				},
				{
					Name:        "unit_name",
					Description: "The sysmted unit name if using systemd as a process manager",
					Type:        tftypes.String,
					Optional:    true,
				},
				{
					Name:            "vault_addr",
					Type:            tftypes.String,
					Required:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description:     "The [api_addr](https://developer.hashicorp.com/vault/docs/configuration#api_addr) of the Vault cluster",
				},
				s.Transport.SchemaAttributeTransport(supportsSSH | supportsK8s | supportsNomad),
				// Input args
				{
					Name:            "key_shares",
					Type:            tftypes.Number,
					Optional:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description:     "The number of [key shares](https://developer.hashicorp.com/vault/docs/commands/operator/init#key-shares)",
				},
				{
					Name:            "key_threshold",
					Type:            tftypes.Number,
					Optional:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description:     "The [key threshold](https://developer.hashicorp.com/vault/docs/commands/operator/init#key-threshold)",
				},
				{
					Name:            "pgp_keys",
					Type:            tftypes.List{ElementType: tftypes.String},
					Optional:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description:     "A list of [pgp keys](https://developer.hashicorp.com/vault/docs/commands/operator/init#pgp-keys)",
				},
				{
					Name:            "root_token_pgp_key",
					Type:            tftypes.String,
					Optional:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description:     "The root token [pgp keys](https://developer.hashicorp.com/vault/docs/commands/operator/init#root-token-pgp-key)",
				},
				{
					Name:            "recovery_shares",
					Type:            tftypes.Number,
					Optional:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description:     "The number of [recovery shares](https://developer.hashicorp.com/vault/docs/commands/operator/init#recovery-shares)",
				},
				{
					Name:            "recovery_threshold",
					Type:            tftypes.Number,
					Optional:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description:     "The [recovery threshold](https://developer.hashicorp.com/vault/docs/commands/operator/init#recovery-threshold)",
				},
				{
					Name:            "recovery_pgp_keys",
					Type:            tftypes.List{ElementType: tftypes.String},
					Optional:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description:     "A list of [recovery pgp keys](https://developer.hashicorp.com/vault/docs/commands/operator/init#recovery-pgp-keys)",
				},
				{
					Name:            "stored_shares",
					Type:            tftypes.Number,
					Optional:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description:     "The number of [stored shares](https://developer.hashicorp.com/vault/docs/commands/operator/init#stored-shares)",
				},
				{
					Name:            "consul_auto",
					Type:            tftypes.Bool,
					Optional:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description:     "Whether to enable or disable [consul auto discovery](https://developer.hashicorp.com/vault/docs/commands/operator/init#consul-auto)",
				},
				{
					Name:            "consul_service",
					Type:            tftypes.String,
					Optional:        true,
					DescriptionKind: tfprotov6.StringKindMarkdown,
					Description:     "The name of the [consul service](https://developer.hashicorp.com/vault/docs/commands/operator/init#consul-service)",
				},
				// outputs
				{
					Name:        "unseal_keys_b64",
					Type:        tftypes.List{ElementType: tftypes.String},
					Optional:    true,
					Computed:    true,
					Description: "The generated unseal keys in base 64",
				},
				{
					Name:        "unseal_keys_hex",
					Type:        tftypes.List{ElementType: tftypes.String},
					Optional:    true,
					Computed:    true,
					Description: "The generated unseal keys in hex",
				},
				{
					Name:        "unseal_keys_shares",
					Type:        tftypes.Number,
					Optional:    true,
					Computed:    true,
					Description: "The number of unseal key shares",
				},
				{
					Name:        "unseal_keys_threshold",
					Type:        tftypes.Number,
					Optional:    true,
					Computed:    true,
					Description: "The number of unseal key shares required to unseal",
				},
				{
					Name:        "recovery_keys_b64",
					Type:        tftypes.List{ElementType: tftypes.String},
					Optional:    true,
					Computed:    true,
					Description: "The root token",
				},
				{
					Name:        "recovery_keys_hex",
					Type:        tftypes.List{ElementType: tftypes.String},
					Optional:    true,
					Computed:    true,
					Description: "The generated recovery keys in base 64",
				},
				{
					Name: "recovery_keys_shares",
					Type: tftypes.Number, Optional: true,
					Computed:    true,
					Description: "The generated recovery keys in hex",
				},
				{
					Name:        "recovery_keys_threshold",
					Type:        tftypes.Number,
					Optional:    true,
					Computed:    true,
					Description: "The number of recovery key shares",
				},
				{
					Name:        "root_token",
					Type:        tftypes.String,
					Optional:    true,
					Computed:    true,
					Description: "The number of recovery key shares required to recovery",
				},
			},
		},
	}
}

// Validate validates the configuration. This will validate the source file
// exists and that the transport configuration is valid.
func (s *vaultInitStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return s.buildInitRequest().Validate()
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *vaultInitStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id":         s.ID,
		"bin_path":   s.BinPath,
		"vault_addr": s.VaultAddr,
		"unit_name":  s.SystemdUnitName,
		// inputs
		"key_shares":         s.KeyShares,
		"key_threshold":      s.KeyThreshold,
		"pgp_keys":           s.PGPKeys,
		"root_token_pgp_key": s.RootTokenPGPKey,
		"recovery_shares":    s.RecoveryShares,
		"recovery_threshold": s.RecoveryThreshold,
		"recovery_pgp_keys":  s.RecoveryPGPKeys,
		"stored_shares":      s.StoredShares,
		"consul_auto":        s.ConsulAuto,
		"consul_service":     s.ConsulService,
		// outputs
		"unseal_keys_shares":      s.UnsealShares,
		"unseal_keys_threshold":   s.UnsealThreshold,
		"unseal_keys_b64":         s.UnsealKeysB64,
		"unseal_keys_hex":         s.UnsealKeysHex,
		"recovery_keys_shares":    s.RecoveryKeysShares,
		"recovery_keys_threshold": s.RecoveryKeysThreshold,
		"recovery_keys_b64":       s.RecoveryKeysB64,
		"recovery_keys_hex":       s.RecoveryKeysHex,
		"root_token":              s.RootToken,
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
func (s *vaultInitStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":         s.ID.TFType(),
		"bin_path":   s.BinPath.TFType(),
		"unit_name":  s.SystemdUnitName.TFType(),
		"transport":  s.Transport.Terraform5Type(),
		"vault_addr": s.VaultAddr.TFType(),
		// inputs
		"key_shares":         s.KeyShares.TFType(),
		"key_threshold":      s.KeyThreshold.TFType(),
		"pgp_keys":           s.PGPKeys.TFType(),
		"root_token_pgp_key": s.RootTokenPGPKey.TFType(),
		"recovery_shares":    s.RecoveryShares.TFType(),
		"recovery_threshold": s.RecoveryThreshold.TFType(),
		"recovery_pgp_keys":  s.RecoveryPGPKeys.TFType(),
		"stored_shares":      s.StoredShares.TFType(),
		"consul_auto":        s.ConsulAuto.TFType(),
		"consul_service":     s.ConsulService.TFType(),
		// outputs
		"unseal_keys_b64":         s.UnsealKeysB64.TFType(),
		"unseal_keys_hex":         s.UnsealKeysHex.TFType(),
		"unseal_keys_shares":      s.UnsealShares.TFType(),
		"unseal_keys_threshold":   s.UnsealThreshold.TFType(),
		"recovery_keys_b64":       s.RecoveryKeysB64.TFType(),
		"recovery_keys_hex":       s.RecoveryKeysHex.TFType(),
		"recovery_keys_shares":    s.RecoveryShares.TFType(),
		"recovery_keys_threshold": s.RecoveryKeysThreshold.TFType(),
		"root_token":              s.RootToken.TFType(),
	}}
}

// Terraform5Value is the file state tftypes.Value.
func (s *vaultInitStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":         s.ID.TFValue(),
		"bin_path":   s.BinPath.TFValue(),
		"transport":  s.Transport.Terraform5Value(),
		"unit_name":  s.SystemdUnitName.TFValue(),
		"vault_addr": s.VaultAddr.TFValue(),
		// inputs
		"key_shares":         s.KeyShares.TFValue(),
		"key_threshold":      s.KeyThreshold.TFValue(),
		"pgp_keys":           s.PGPKeys.TFValue(),
		"root_token_pgp_key": s.RootTokenPGPKey.TFValue(),
		"recovery_shares":    s.RecoveryShares.TFValue(),
		"recovery_threshold": s.RecoveryThreshold.TFValue(),
		"recovery_pgp_keys":  s.RecoveryPGPKeys.TFValue(),
		"stored_shares":      s.StoredShares.TFValue(),
		"consul_auto":        s.ConsulAuto.TFValue(),
		"consul_service":     s.ConsulService.TFValue(),
		// outputs
		"unseal_keys_b64":         s.UnsealKeysB64.TFValue(),
		"unseal_keys_hex":         s.UnsealKeysHex.TFValue(),
		"unseal_keys_shares":      s.UnsealShares.TFValue(),
		"unseal_keys_threshold":   s.UnsealThreshold.TFValue(),
		"recovery_keys_b64":       s.RecoveryKeysB64.TFValue(),
		"recovery_keys_hex":       s.RecoveryKeysHex.TFValue(),
		"recovery_keys_shares":    s.RecoveryKeysShares.TFValue(),
		"recovery_keys_threshold": s.RecoveryKeysThreshold.TFValue(),
		"root_token":              s.RootToken.TFValue(),
	})
}

// EmbeddedTransport returns a pointer the resources embedded transport.
func (s *vaultInitStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}

// Init initializes a vault cluster.
func (s *vaultInitStateV1) Init(ctx context.Context, client it.Transport) error {
	req := s.buildInitRequest()
	err := req.Validate()
	if err != nil {
		return fmt.Errorf("failed to initialize vault because init request validation failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	res, err := vault.Init(ctx, client, req)
	if err != nil {
		err = fmt.Errorf("failed to initialize the vault cluster: %w", err)
		if res.PriorState != nil {
			err = fmt.Errorf(
				"%w\nVault State before running '%s'\n%s",
				err, req.String(), istrings.Indent("  ", res.PriorState.String()),
			)
		}
		if res.PostState != nil {
			err = fmt.Errorf(
				"%w\nVault State after running '%s'\n%s",
				err, req.String(), istrings.Indent("  ", res.PriorState.String()),
			)
		}

		return err
	}

	// Migrate the init response to the state
	s.UnsealKeysB64.SetStrings(res.UnsealKeysB64)
	s.UnsealKeysHex.SetStrings(res.UnsealKeysHex)
	shares, err := res.UnsealShares.Int64()
	if err == nil {
		s.UnsealShares.Set(int(shares))
	} else {
		s.UnsealShares.Null = true
		s.UnsealShares.Unknown = false
	}
	thresh, err := res.UnsealThreshold.Int64()
	if err == nil {
		s.UnsealThreshold.Set(int(thresh))
	} else {
		s.UnsealThreshold.Null = true
		s.UnsealThreshold.Unknown = false
	}
	s.RecoveryKeysB64.SetStrings(res.RecoveryKeysB64)
	s.RecoveryKeysHex.SetStrings(res.RecoveryKeysHex)
	shares, err = res.RecoveryKeysShares.Int64()
	if err == nil {
		s.RecoveryKeysShares.Set(int(shares))
	} else {
		s.RecoveryKeysShares.Null = true
		s.RecoveryKeysShares.Unknown = false
	}
	thresh, err = res.RecoveryKeysThreshold.Int64()
	if err == nil {
		s.RecoveryKeysThreshold.Set(int(thresh))
	} else {
		s.RecoveryKeysThreshold.Null = true
		s.RecoveryKeysThreshold.Unknown = false
	}
	s.RootToken.Set(res.RootToken)

	return nil
}

func (s *vaultInitStateV1) buildInitRequest() *vault.InitRequest {
	stateOpts := []vault.StateRequestOpt{
		vault.WithStateRequestFlightControlUseHomeDir(),
	}

	if binPath, ok := s.BinPath.Get(); ok {
		stateOpts = append(stateOpts, vault.WithStateRequestBinPath(binPath))
	}

	if vaultAddr, ok := s.VaultAddr.Get(); ok {
		stateOpts = append(stateOpts, vault.WithStateRequestVaultAddr(vaultAddr))
	}

	unitName := "vault"
	if unit, ok := s.SystemdUnitName.Get(); ok {
		unitName = unit
	}
	stateOpts = append(stateOpts, vault.WithStateRequestSystemdUnitName(unitName))

	initOpts := []vault.InitRequestOpt{
		vault.WithInitRequestStateRequestOpts(stateOpts...),
	}

	if keyShares, ok := s.KeyShares.Get(); ok {
		initOpts = append(initOpts, vault.WithInitRequestKeyShares(keyShares))
	}

	if keyThreshold, ok := s.KeyThreshold.Get(); ok {
		initOpts = append(initOpts, vault.WithInitRequestKeyThreshold(keyThreshold))
	}

	if pgpKeys, ok := s.PGPKeys.GetStrings(); ok {
		initOpts = append(initOpts, vault.WithInitRequestPGPKeys(pgpKeys))
	}

	if rootTokenPGPKey, ok := s.RootTokenPGPKey.Get(); ok {
		initOpts = append(initOpts, vault.WithInitRequestRootTokenPGPKey(rootTokenPGPKey))
	}

	if recoveryShares, ok := s.RecoveryShares.Get(); ok {
		initOpts = append(initOpts, vault.WithInitRequestRecoveryShares(recoveryShares))
	}

	if recoveryThreshold, ok := s.RecoveryThreshold.Get(); ok {
		initOpts = append(initOpts, vault.WithInitRequestRecoveryThreshold(recoveryThreshold))
	}

	if recoveryPGPKeys, ok := s.RecoveryPGPKeys.GetStrings(); ok {
		initOpts = append(initOpts, vault.WithInitRequestRecoveryPGPKeys(recoveryPGPKeys))
	}

	if storedShares, ok := s.StoredShares.Get(); ok {
		initOpts = append(initOpts, vault.WithInitRequestStoredShares(storedShares))
	}

	if consulAuto, ok := s.ConsulAuto.Get(); ok {
		initOpts = append(initOpts, vault.WithInitRequestConsulAuto(consulAuto))
	}

	if consulSvc, ok := s.ConsulService.Get(); ok {
		initOpts = append(initOpts, vault.WithInitRequestConsulService(consulSvc))
	}

	return vault.NewInitRequest(initOpts...)
}
