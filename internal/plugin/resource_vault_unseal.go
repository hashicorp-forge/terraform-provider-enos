package plugin

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/hashicorp/enos-provider/internal/diags"
	"github.com/hashicorp/enos-provider/internal/remoteflight/vault"
	resource "github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/enos-provider/internal/server/state"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type vaultUnseal struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resource.Resource = (*vaultUnseal)(nil)

type vaultUnsealStateV1 struct {
	ID         *tfString
	BinPath    *tfString
	VaultAddr  *tfString
	SealType   *tfString
	UnsealKeys *tfStringSlice
	Status     *tfNum
	Transport  *embeddedTransportV1

	resolvedTransport transportState
}

var _ state.State = (*vaultUnsealStateV1)(nil)

func newVaultUnseal() *vaultUnseal {
	return &vaultUnseal{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newVaultUnsealStateV1() *vaultUnsealStateV1 {
	return &vaultUnsealStateV1{
		ID:         newTfString(),
		BinPath:    newTfString(),
		VaultAddr:  newTfString(),
		SealType:   newTfString(),
		UnsealKeys: newTfStringSlice(),
		Transport:  newEmbeddedTransport(),
	}
}

func (r *vaultUnseal) Name() string {
	return "enos_vault_unseal"
}

func (r *vaultUnseal) Schema() *tfprotov6.Schema {
	return newVaultUnsealStateV1().Schema()
}

func (r *vaultUnseal) SetProviderConfig(meta tftypes.Value) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.FromTerraform5Value(meta)
}

func (r *vaultUnseal) GetProviderConfig() (*config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.Copy()
}

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (r *vaultUnseal) ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	newState := newVaultUnsealStateV1()

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
func (r *vaultUnseal) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	newState := newVaultUnsealStateV1()

	transportUtil.UpgradeResourceState(ctx, newState, req, res)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *vaultUnseal) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	newState := newVaultUnsealStateV1()

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
func (r *vaultUnseal) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	newState := newVaultUnsealStateV1()

	transportUtil.ImportResourceState(ctx, newState, req, res)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *vaultUnseal) PlanResourceChange(ctx context.Context, req resource.PlanResourceChangeRequest, res *resource.PlanResourceChangeResponse) {
	priorState := newVaultUnsealStateV1()
	proposedState := newVaultUnsealStateV1()
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
func (r *vaultUnseal) ApplyResourceChange(ctx context.Context, req resource.ApplyResourceChangeRequest, res *resource.ApplyResourceChangeResponse) {
	priorState := newVaultUnsealStateV1()
	plannedState := newVaultUnsealStateV1()
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
	defer client.Close() //nolint: staticcheck

	// If our priorState ID is blank then we're creating the resource
	if _, ok := priorState.ID.Get(); !ok {
		err = plannedState.Unseal(ctx, client)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Vault Unseal Error", err))
			return
		}
	} else if !reflect.DeepEqual(plannedState, priorState) {
		err = plannedState.Unseal(ctx, client)

		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Vault Unseal Error", err))
			return
		}
	}
}

// Schema is the file states Terraform schema.
func (s *vaultUnsealStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:     "id",
					Type:     s.ID.TFType(),
					Computed: true,
				},
				{
					Name:     "bin_path",
					Type:     s.BinPath.TFType(),
					Required: true,
				},
				{
					Name:     "vault_addr",
					Type:     s.VaultAddr.TFType(),
					Required: true,
				},
				{
					Name:     "seal_type",
					Type:     s.SealType.TFType(),
					Optional: true,
				},
				{
					Name:      "unseal_keys",
					Type:      s.UnsealKeys.TFType(),
					Required:  true,
					Sensitive: true,
				},
				s.Transport.SchemaAttributeTransport(),
			},
		},
	}
}

// Validate validates the configuration. This will validate the source file
// exists and that the transport configuration is valid.
func (s *vaultUnsealStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, ok := s.BinPath.Get(); !ok {
		return ValidationError("you must provide the Vault bin path", "bin_path")
	}

	if _, ok := s.VaultAddr.Get(); !ok {
		return ValidationError("you must provide the Vault address", "vault_addr")
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *vaultUnsealStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id":          s.ID,
		"bin_path":    s.BinPath,
		"vault_addr":  s.VaultAddr,
		"seal_type":   s.SealType,
		"unseal_keys": s.UnsealKeys,
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
func (s *vaultUnsealStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":          s.ID.TFType(),
		"bin_path":    s.BinPath.TFType(),
		"vault_addr":  s.VaultAddr.TFType(),
		"seal_type":   s.SealType.TFType(),
		"unseal_keys": s.UnsealKeys.TFType(),
		"transport":   s.Transport.Terraform5Type(),
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (s *vaultUnsealStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":          s.ID.TFValue(),
		"bin_path":    s.BinPath.TFValue(),
		"vault_addr":  s.VaultAddr.TFValue(),
		"seal_type":   s.SealType.TFValue(),
		"unseal_keys": s.UnsealKeys.TFValue(),
		"transport":   s.Transport.Terraform5Value(),
	})
}

// EmbeddedTransport returns a pointer the resources embedded transport.
func (s *vaultUnsealStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}

func (s *vaultUnsealStateV1) Unseal(ctx context.Context, client it.Transport) error {
	req := s.buildUnsealRequest()
	err := vault.Unseal(ctx, client, req)
	if err != nil {
		return fmt.Errorf("failed to unseal Vault, due to: %w", err)
	}
	return err
}

func (s *vaultUnsealStateV1) setResolvedTransport(transport transportState) {
	s.resolvedTransport = transport
}

func (s *vaultUnsealStateV1) Debug() string {
	if s.resolvedTransport == nil {
		return s.EmbeddedTransport().Debug()
	}
	return s.resolvedTransport.debug()
}

func (s *vaultUnsealStateV1) buildUnsealRequest() *vault.UnsealRequest {
	opts := []vault.UnsealRequestOpt{
		vault.WithUnsealRequestBinPath(s.BinPath.Value()),
		vault.WithUnsealRequestVaultAddr(s.VaultAddr.Value()),
	}
	// If sealtype unset, defaults to shamir
	if seal, ok := s.SealType.Get(); !ok {
		opts = append(opts, vault.WithUnsealRequestSealType(vault.SealTypeShamir))
	} else {
		opts = append(opts, vault.WithUnsealRequestSealType(vault.SealType(seal)))
	}
	unsealKeys, ok := s.UnsealKeys.GetStrings()
	if ok {
		opts = append(opts, vault.WithUnsealRequestUnsealKeys(unsealKeys))
	}

	return vault.NewUnsealRequest(opts...)
}
