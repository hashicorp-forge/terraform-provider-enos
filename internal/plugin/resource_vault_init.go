package plugin

import (
	"context"
	"reflect"
	"sync"

	"github.com/hashicorp/enos-provider/internal/flightcontrol/remoteflight/vault"
	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type vaultInit struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resourcerouter.Resource = (*vaultInit)(nil)

type vaultInitStateV1 struct {
	ID        string
	BinPath   string
	VaultAddr string
	Transport *embeddedTransportV1
	// inputs
	KeyShares         *tfNum
	KeyThreshold      *tfNum
	PGPKeys           *tfStringSlice
	RecoveryShares    *tfNum
	RecoveryThreshold *tfNum
	RecoveryPGPKeys   *tfStringSlice
	RootTokenPGPKey   string
	ConsulAuto        *tfBool
	ConsulService     string
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
	RootToken             string
}

var _ State = (*vaultInitStateV1)(nil)

func newVaultInit() *vaultInit {
	return &vaultInit{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newVaultInitStateV1() *vaultInitStateV1 {
	return &vaultInitStateV1{
		Transport: newEmbeddedTransport(),
		// inputs
		KeyShares:         &tfNum{},
		KeyThreshold:      &tfNum{},
		PGPKeys:           &tfStringSlice{},
		RecoveryShares:    &tfNum{},
		RecoveryThreshold: &tfNum{},
		RecoveryPGPKeys:   &tfStringSlice{},
		ConsulAuto:        &tfBool{},
		StoredShares:      &tfNum{},
		// outputs
		UnsealKeysB64:         &tfStringSlice{},
		UnsealKeysHex:         &tfStringSlice{},
		UnsealShares:          &tfNum{},
		UnsealThreshold:       &tfNum{},
		RecoveryKeysB64:       &tfStringSlice{},
		RecoveryKeysHex:       &tfStringSlice{},
		RecoveryKeysShares:    &tfNum{},
		RecoveryKeysThreshold: &tfNum{},
	}
}

func (r *vaultInit) Name() string {
	return "enos_vault_init"
}

func (r *vaultInit) Schema() *tfprotov5.Schema {
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

// ValidateResourceTypeConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (r *vaultInit) ValidateResourceTypeConfig(ctx context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	newState := newVaultInitStateV1()

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
func (r *vaultInit) UpgradeResourceState(ctx context.Context, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
	newState := newVaultInitStateV1()

	return transportUtil.UpgradeResourceState(ctx, newState, req)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *vaultInit) ReadResource(ctx context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	newState := newVaultInitStateV1()

	return transportUtil.ReadResource(ctx, newState, req)
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
func (r *vaultInit) ImportResourceState(ctx context.Context, req *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
	newState := newVaultInitStateV1()

	return transportUtil.ImportResourceState(ctx, newState, req)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *vaultInit) PlanResourceChange(ctx context.Context, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, error) {
	priorState := newVaultInitStateV1()
	proposedState := newVaultInitStateV1()

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
		proposedState.UnsealKeysB64.unknown = true
		proposedState.UnsealKeysHex.unknown = true
		proposedState.UnsealShares.unknown = true
		proposedState.UnsealThreshold.unknown = true
		proposedState.RecoveryKeysB64.unknown = true
		proposedState.RecoveryKeysHex.unknown = true
		proposedState.RootToken = UnknownString
	}

	err = transportUtil.PlanMarshalPlannedState(ctx, res, proposedState, transport)

	return res, err
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *vaultInit) ApplyResourceChange(ctx context.Context, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
	priorState := newVaultInitStateV1()
	plannedState := newVaultInitStateV1()

	res, err := transportUtil.ApplyUnmarshalState(ctx, priorState, plannedState, req)
	if err != nil {
		return res, err
	}

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

	if !reflect.DeepEqual(priorState, plannedState) {
		err = plannedState.Init(ctx, ssh)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
	}
	err = transportUtil.ApplyMarshalNewState(ctx, res, plannedState, transport)

	return res, err
}

// Schema is the file states Terraform schema.
func (s *vaultInitStateV1) Schema() *tfprotov5.Schema {
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
					Name:     "bin_path",
					Type:     tftypes.String,
					Required: true,
				},
				{
					Name:     "vault_addr",
					Type:     tftypes.String,
					Required: true,
				},
				s.Transport.SchemaAttributeTransport(),
				// Input args
				{
					Name:     "key_shares",
					Type:     tftypes.Number,
					Optional: true,
				},
				{
					Name:     "key_threshold",
					Type:     tftypes.Number,
					Optional: true,
				},
				{
					Name:     "pgp_keys",
					Type:     tftypes.List{ElementType: tftypes.String},
					Optional: true,
				},
				{
					Name:     "root_token_pgp_key",
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:     "recovery_shares",
					Type:     tftypes.Number,
					Optional: true,
				},
				{
					Name:     "recovery_threshold",
					Type:     tftypes.Number,
					Optional: true,
				},
				{
					Name:     "recovery_pgp_keys",
					Type:     tftypes.List{ElementType: tftypes.String},
					Optional: true,
				},
				{
					Name:     "stored_shares",
					Type:     tftypes.Number,
					Optional: true,
				},
				{
					Name:     "consul_auto",
					Type:     tftypes.Bool,
					Optional: true,
				},
				{
					Name:     "consul_service",
					Type:     tftypes.String,
					Optional: true,
				},
				// outputs
				{
					Name:     "unseal_keys_b64",
					Type:     tftypes.List{ElementType: tftypes.String},
					Optional: true,
					Computed: true,
				},
				{
					Name:     "unseal_keys_hex",
					Type:     tftypes.List{ElementType: tftypes.String},
					Optional: true,
					Computed: true,
				},
				{
					Name:     "unseal_keys_shares",
					Type:     tftypes.Number,
					Optional: true,
					Computed: true,
				},
				{
					Name:     "unseal_keys_threshold",
					Type:     tftypes.Number,
					Optional: true,
					Computed: true,
				},
				{
					Name:     "recovery_keys_b64",
					Type:     tftypes.List{ElementType: tftypes.String},
					Optional: true,
					Computed: true,
				},
				{
					Name:     "recovery_keys_hex",
					Type:     tftypes.List{ElementType: tftypes.String},
					Optional: true,
					Computed: true,
				},
				{
					Name: "recovery_keys_shares",
					Type: tftypes.Number, Optional: true,
					Computed: true,
				},
				{
					Name:     "recovery_keys_threshold",
					Type:     tftypes.Number,
					Optional: true,
					Computed: true,
				},
				{
					Name:     "root_token",
					Type:     tftypes.String,
					Optional: true,
					Computed: true,
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
		"id":         &s.ID,
		"bin_path":   &s.BinPath,
		"vault_addr": &s.VaultAddr,
		// inputs
		"key_shares":         s.KeyShares,
		"key_threshold":      s.KeyThreshold,
		"pgp_keys":           s.PGPKeys,
		"root_token_pgp_key": &s.RootTokenPGPKey,
		"recovery_shares":    s.RecoveryShares,
		"recovery_threshold": s.RecoveryThreshold,
		"recovery_pgp_keys":  s.RecoveryPGPKeys,
		"stored_shares":      s.StoredShares,
		"consul_auto":        s.ConsulAuto,
		"consul_service":     &s.ConsulService,
		// outputs
		"unseal_keys_shares":      s.UnsealShares,
		"unseal_keys_threshold":   s.UnsealThreshold,
		"unseal_keys_b64":         s.UnsealKeysB64,
		"unseal_keys_hex":         s.UnsealKeysHex,
		"recovery_keys_shares":    s.RecoveryKeysShares,
		"recovery_keys_threshold": s.RecoveryKeysThreshold,
		"recovery_keys_b64":       s.RecoveryKeysB64,
		"recovery_keys_hex":       s.RecoveryKeysHex,
		"root_token":              &s.RootToken,
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
		"id":         tftypes.String,
		"bin_path":   tftypes.String,
		"vault_addr": tftypes.String,
		"transport":  s.Transport.Terraform5Type(),
		// inputs
		"key_shares":         s.KeyShares.TFType(),
		"key_threshold":      s.KeyThreshold.TFType(),
		"pgp_keys":           s.PGPKeys.TFType(),
		"root_token_pgp_key": tftypes.String,
		"recovery_shares":    s.RecoveryShares.TFType(),
		"recovery_threshold": s.RecoveryThreshold.TFType(),
		"recovery_pgp_keys":  s.RecoveryPGPKeys.TFType(),
		"stored_shares":      s.StoredShares.TFType(),
		"consul_auto":        s.ConsulAuto.TFType(),
		"consul_service":     tftypes.String,
		// outputs
		"unseal_keys_b64":         s.UnsealKeysB64.TFType(),
		"unseal_keys_hex":         s.UnsealKeysHex.TFType(),
		"unseal_keys_shares":      s.UnsealShares.TFType(),
		"unseal_keys_threshold":   s.UnsealThreshold.TFType(),
		"recovery_keys_b64":       s.RecoveryKeysB64.TFType(),
		"recovery_keys_hex":       s.RecoveryKeysHex.TFType(),
		"recovery_keys_shares":    s.RecoveryShares.TFType(),
		"recovery_keys_threshold": s.RecoveryKeysThreshold.TFType(),
		"root_token":              tftypes.String,
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (s *vaultInitStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":         tfMarshalStringValue(s.ID),
		"bin_path":   tfMarshalStringValue(s.BinPath),
		"vault_addr": tfMarshalStringValue(s.VaultAddr),
		"transport":  s.Transport.Terraform5Value(),
		// inputs
		"key_shares":         s.KeyShares.TFValue(),
		"key_threshold":      s.KeyThreshold.TFValue(),
		"pgp_keys":           s.PGPKeys.TFValue(),
		"root_token_pgp_key": tfMarshalStringOptionalValue(s.RootTokenPGPKey),
		"recovery_shares":    s.RecoveryShares.TFValue(),
		"recovery_threshold": s.RecoveryThreshold.TFValue(),
		"recovery_pgp_keys":  s.RecoveryPGPKeys.TFValue(),
		"stored_shares":      s.StoredShares.TFValue(),
		"consul_auto":        s.ConsulAuto.TFValue(),
		"consul_service":     tfMarshalStringOptionalValue(s.ConsulService),
		// outputs
		"unseal_keys_b64":         s.UnsealKeysB64.TFValue(),
		"unseal_keys_hex":         s.UnsealKeysHex.TFValue(),
		"unseal_keys_shares":      s.UnsealShares.TFValue(),
		"unseal_keys_threshold":   s.UnsealThreshold.TFValue(),
		"recovery_keys_b64":       s.RecoveryKeysB64.TFValue(),
		"recovery_keys_hex":       s.RecoveryKeysHex.TFValue(),
		"recovery_keys_shares":    s.RecoveryKeysShares.TFValue(),
		"recovery_keys_threshold": s.RecoveryKeysThreshold.TFValue(),
		"root_token":              tfMarshalStringValue(s.RootToken),
	})
}

// EmbeddedTransport returns a pointer the resources embedded transport.
func (s *vaultInitStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}

// Init initializes a vault cluster
func (s *vaultInitStateV1) Init(ctx context.Context, ssh it.Transport) error {
	req := s.buildInitRequest()
	err := req.Validate()
	if err != nil {
		return wrapErrWithDiagnostics(err, "init request", "validating vault init request")
	}

	res, err := vault.Init(ctx, ssh, req)
	if err != nil {
		return wrapErrWithDiagnostics(err, "vault init", "initializing vault cluster")
	}

	// Migrate the init response to the state
	s.UnsealKeysB64.Set(res.UnsealKeysB64)
	s.UnsealKeysHex.Set(res.UnsealKeysHex)
	shares, err := res.UnsealShares.Int64()
	if err != nil {
		s.UnsealShares.Set(int(shares))
	} else {
		s.UnsealShares.null = true
		s.UnsealShares.unknown = false
	}
	thresh, err := res.UnsealThreshold.Int64()
	if err != nil {
		s.UnsealThreshold.Set(int(thresh))
	} else {
		s.UnsealThreshold.null = true
		s.UnsealThreshold.unknown = false
	}
	s.RecoveryKeysB64.Set(res.RecoveryKeysB64)
	s.RecoveryKeysHex.Set(res.RecoveryKeysHex)
	shares, err = res.RecoveryKeysShares.Int64()
	if err != nil {
		s.RecoveryKeysShares.Set(int(shares))
	} else {
		s.RecoveryKeysShares.null = true
		s.RecoveryKeysShares.unknown = false
	}
	thresh, err = res.RecoveryKeysThreshold.Int64()
	if err != nil {
		s.RecoveryKeysThreshold.Set(int(thresh))
	} else {
		s.RecoveryKeysThreshold.null = true
		s.RecoveryKeysThreshold.unknown = false
	}
	s.RootToken = res.RootToken

	return nil
}

func (s *vaultInitStateV1) buildInitRequest() *vault.InitRequest {
	opts := []vault.InitRequestOpt{
		vault.WithInitRequestBinPath(s.BinPath),
		vault.WithInitRequestVaultAddr(s.VaultAddr),
	}

	keyShares, ok := s.KeyShares.Get()
	if ok {
		opts = append(opts, vault.WithInitRequestKeyShares(keyShares))
	}

	keyThreshold, ok := s.KeyThreshold.Get()
	if ok {
		opts = append(opts, vault.WithInitRequestKeyThreshold(keyThreshold))
	}

	pgpKeys, ok := s.PGPKeys.Get()
	if ok {
		opts = append(opts, vault.WithInitRequestPGPKeys(pgpKeys))
	}

	if s.RootTokenPGPKey != "" && s.RootTokenPGPKey != UnknownString {
		opts = append(opts, vault.WithInitRequestRootTokenPGPKey(s.RootTokenPGPKey))
	}

	recoveryShares, ok := s.RecoveryShares.Get()
	if ok {
		opts = append(opts, vault.WithInitRequestRecoveryShares(recoveryShares))
	}

	recoveryThreshold, ok := s.RecoveryThreshold.Get()
	if ok {
		opts = append(opts, vault.WithInitRequestRecoveryThreshold(recoveryThreshold))
	}

	recoveryPGPKeys, ok := s.RecoveryPGPKeys.Get()
	if ok {
		opts = append(opts, vault.WithInitRequestRecoveryPGPKeys(recoveryPGPKeys))
	}

	storedShares, ok := s.StoredShares.Get()
	if ok {
		opts = append(opts, vault.WithInitRequestStoredShares(storedShares))
	}

	consulAuto, ok := s.ConsulAuto.Get()
	if ok {
		opts = append(opts, vault.WithInitRequestConsulAuto(consulAuto))
	}

	if s.ConsulService != "" && s.ConsulService != UnknownString {
		opts = append(opts, vault.WithInitRequestConsulService(s.ConsulService))
	}

	return vault.NewInitRequest(opts...)
}
