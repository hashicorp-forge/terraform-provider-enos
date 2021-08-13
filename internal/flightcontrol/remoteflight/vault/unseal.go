package vault

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/enos-provider/internal/flightcontrol/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// UnsealRequest is a Vault unseal request
type UnsealRequest struct {
	*CLIRequest
	*UnsealArguments
}

type UnsealArguments struct {
	SealType   SealType `json:"seal_type"`
	UnsealKeys []string `json:"unseal_keys"`
}

// SealType is the Vault seal type
type SealType string

// SealTypes are the possible Vault seal types
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

// UnsealRequestOpt is a functional option for a unseal request
type UnsealRequestOpt func(*UnsealRequest) *UnsealRequest

// NewUnsealRequest takes functional options and returns a new
// unseal request
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

// WithUnsealRequestBinPath sets the Vault binary path
func WithUnsealRequestBinPath(path string) UnsealRequestOpt {
	return func(u *UnsealRequest) *UnsealRequest {
		u.BinPath = path
		return u
	}
}

// WithUnsealRequestVaultAddr sets the Vault address
func WithUnsealRequestVaultAddr(addr string) UnsealRequestOpt {
	return func(u *UnsealRequest) *UnsealRequest {
		u.VaultAddr = addr
		return u
	}
}

// WithUnsealRequestSealType sets the Vault seal type
func WithUnsealRequestSealType(typ SealType) UnsealRequestOpt {
	return func(u *UnsealRequest) *UnsealRequest {
		u.SealType = typ
		return u
	}
}

// WithUnsealRequestUnsealKeys sets the Vault unseal keys
func WithUnsealRequestUnsealKeys(unsealKeys []string) UnsealRequestOpt {
	return func(u *UnsealRequest) *UnsealRequest {
		u.UnsealKeys = unsealKeys
		return u
	}
}

// Unseal checks the current steal status, and if needed unseals the Vault in
// different ways depending on seal type
func Unseal(ctx context.Context, ssh it.Transport, req *UnsealRequest) error {
	if req.BinPath == "" {
		return fmt.Errorf("you must supply a Vault bin path")
	}
	if req.VaultAddr == "" {
		return fmt.Errorf("you must supply a Vault listen address")
	}

	status, err := Status(ctx, ssh, NewStatusRequest(WithStatusRequestBinPath(req.BinPath), WithStatusRequestVaultAddr(req.VaultAddr)))
	if err != nil {
		return remoteflight.WrapErrorWith(err, "failed to get Vault status")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(1*time.Minute))
	defer cancel()

	switch status {
	case StatusInitializedUnsealed:
		// Alive, unsealed
		return err
	case StatusError:
		// Connection error, retry
		err = WaitForStatus(timeoutCtx, ssh, NewStatusRequest(
			WithStatusRequestBinPath(req.BinPath),
			WithStatusRequestVaultAddr(req.VaultAddr),
		), StatusInitializedUnsealed)
		if err != nil {
			return remoteflight.WrapErrorWith(err, "timed out waiting for Vault service")
		}
	case StatusSealed:
		// Running but didn't unseal
		switch req.SealType {
		case SealTypeShamir:
			for _, key := range req.UnsealKeys {
				select {
				case <-ctx.Done():
					return fmt.Errorf("timed out unsealing Vault: %w", ctx.Err())
				default:
				}
				_, stderr, err := ssh.Run(ctx, command.New(fmt.Sprintf("%s operator unseal %s;", req.BinPath, key),
					command.WithEnvVar("VAULT_ADDR", req.VaultAddr)))
				if err != nil {
					return fmt.Errorf("failed to unseal: %s stderr: %s", err, stderr)
				}
			}

		case SealTypeAWSKMS:
			// Didn't auto-unseal, restart the service
			err = Restart(timeoutCtx, ssh, NewStatusRequest(
				WithStatusRequestBinPath(req.BinPath),
				WithStatusRequestVaultAddr(req.VaultAddr),
			))
			if err != nil {
				return remoteflight.WrapErrorWith(err, "failed to start the Vault service")
			}
		default:
			return fmt.Errorf("unknown seal type specified")
		}

	default:
		return fmt.Errorf("the Vault service returned an unknown code")
	}
	return err
}
