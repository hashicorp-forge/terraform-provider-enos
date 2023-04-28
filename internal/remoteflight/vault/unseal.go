package vault

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// UnsealRequest is a Vault unseal request.
type UnsealRequest struct {
	*CLIRequest
	*UnsealArguments
}

type UnsealArguments struct {
	SealType   SealType `json:"seal_type"`
	UnsealKeys []string `json:"unseal_keys"`
}

// SealType is the Vault seal type.
type SealType string

// SealTypes are the possible Vault seal types.
const (
	SealTypeShamir        = "shamir"
	SealTypeAliCloud      = "alicloudkms"
	SealTypeAWSKMS        = "awskms"
	SealTypeAzureKeyVault = "azurekeyvault"
	SealTypeGCPKMS        = "gcpkms"
	SealTypeOCIKMS        = "ocikms"
	SealTypeHSMPCKS11     = "pcks11"
	SealTypeTransit       = "transit"
)

// UnsealRequestOpt is a functional option for a unseal request.
type UnsealRequestOpt func(*UnsealRequest) *UnsealRequest

// NewUnsealRequest takes functional options and returns a new
// unseal request.
func NewUnsealRequest(opts ...UnsealRequestOpt) *UnsealRequest {
	c := &UnsealRequest{
		&CLIRequest{},
		&UnsealArguments{
			SealType:   "",
			UnsealKeys: []string{},
		},
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithUnsealRequestBinPath sets the Vault binary path.
func WithUnsealRequestBinPath(path string) UnsealRequestOpt {
	return func(u *UnsealRequest) *UnsealRequest {
		u.BinPath = path
		return u
	}
}

// WithUnsealRequestVaultAddr sets the Vault address.
func WithUnsealRequestVaultAddr(addr string) UnsealRequestOpt {
	return func(u *UnsealRequest) *UnsealRequest {
		u.VaultAddr = addr
		return u
	}
}

// WithUnsealRequestSealType sets the Vault seal type.
func WithUnsealRequestSealType(typ SealType) UnsealRequestOpt {
	return func(u *UnsealRequest) *UnsealRequest {
		u.SealType = typ
		return u
	}
}

// WithUnsealRequestUnsealKeys sets the Vault unseal keys.
func WithUnsealRequestUnsealKeys(unsealKeys []string) UnsealRequestOpt {
	return func(u *UnsealRequest) *UnsealRequest {
		u.UnsealKeys = unsealKeys
		return u
	}
}

// Unseal checks the current steal status, and if needed unseals the Vault in
// different ways depending on seal type.
func Unseal(ctx context.Context, ssh it.Transport, req *UnsealRequest) error {
	binPath := req.BinPath
	if binPath == "" {
		return fmt.Errorf("you must supply a Vault bin path")
	}

	vaultAddr := req.VaultAddr
	if vaultAddr == "" {
		return fmt.Errorf("you must supply a Vault listen address")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	statusRequest := NewStatusRequest(
		WithStatusRequestBinPath(binPath),
		WithStatusRequestVaultAddr(vaultAddr))

	// it is not possible to unseal unless Vault is initialized.
	// We need to wait for Vault to initialize before continuing.
	state, err := WaitForState(timeoutCtx, ssh, statusRequest, CheckIsInitialized())
	if err != nil {
		return fmt.Errorf("cannot unseal vault, vault is not initialized, current state: [%#v], error: %w", state, err)
	}

	switch state.SealStatus {
	case UnSealed:
		// Alive, unsealed
		return nil
	case Error:
		// Connection error, retry, we will wait for Vault to be unsealed below.
	case Sealed:
		// Running but didn't unseal
		switch req.SealType {
		case SealTypeShamir:
			for _, key := range req.UnsealKeys {
				select {
				case <-ctx.Done():
					return fmt.Errorf("timed out unsealing Vault: %w", ctx.Err())
				default:
				}
				_, stderr, err := ssh.Run(ctx, command.New(fmt.Sprintf("%s operator unseal %s;", binPath, key),
					command.WithEnvVar("VAULT_ADDR", vaultAddr)))
				if err != nil {
					return fmt.Errorf("failed to unseal: %s stderr: %s", err, stderr)
				}
			}

		case SealTypeAWSKMS:
			// Didn't auto-unseal yet, we will wait for Vault to be unsealed below
		default:
			return fmt.Errorf("unknown seal type specified")
		}
	case StatusUnknown:
		return fmt.Errorf("the Vault service returned an unknown code")
	default:
		return fmt.Errorf("the Vault service returned an unknown code")
	}

	if state, err = WaitForState(ctx, ssh, statusRequest, CheckIsUnsealed()); err != nil {
		tflog.Error(ctx, "Failed to unseal vault", map[string]interface{}{"state": state})
		return remoteflight.WrapErrorWith(err, "failed to unseal")
	}

	return nil
}
