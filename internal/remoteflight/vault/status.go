package vault

import (
	"context"
	"errors"
	"fmt"
	"time"

	xssh "golang.org/x/crypto/ssh"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// StatusCode are vault status exit codes
type StatusCode int

// Vault status exit codes
const (
	StatusInitializedUnsealed StatusCode = 0
	StatusError               StatusCode = 1
	StatusSealed              StatusCode = 2
	// StatusUnknown is returned if a non-vault status error code is encountered
	StatusUnknown StatusCode = 9
)

// StatusRequest is a vault status request
type StatusRequest struct {
	*CLIRequest
}

// StatusRequestOpt is a functional option for a config create request
type StatusRequestOpt func(*StatusRequest) *StatusRequest

// NewStatusRequest takes functional options and returns a new
// systemd unit request
func NewStatusRequest(opts ...StatusRequestOpt) *StatusRequest {
	c := &StatusRequest{
		&CLIRequest{},
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithStatusRequestBinPath sets the vault binary path
func WithStatusRequestBinPath(path string) StatusRequestOpt {
	return func(u *StatusRequest) *StatusRequest {
		u.BinPath = path
		return u
	}
}

// WithStatusRequestVaultAddr sets the vault address
func WithStatusRequestVaultAddr(addr string) StatusRequestOpt {
	return func(u *StatusRequest) *StatusRequest {
		u.VaultAddr = addr
		return u
	}
}

// Status returns the vault status code
func Status(ctx context.Context, ssh it.Transport, req *StatusRequest) (StatusCode, error) {
	if req.BinPath == "" {
		return StatusUnknown, fmt.Errorf("you must supply a vault bin path")
	}
	if req.VaultAddr == "" {
		return StatusUnknown, fmt.Errorf("you must supply a vault listen address")
	}

	_, stderr, err := ssh.Run(ctx, command.New(
		fmt.Sprintf("%s status", req.BinPath),
		command.WithEnvVar("VAULT_ADDR", req.VaultAddr),
	))
	// If we don't get an error we're initialized and unsealed
	if err == nil {
		return StatusInitializedUnsealed, nil
	}

	// Determine what the error status is and if we need to return an error to
	// the caller.
	statusCode := StatusUnknown
	var exitError *xssh.ExitError
	if errors.As(err, &exitError) {
		statusCode = StatusCode(exitError.Waitmsg.ExitStatus())
	}

	switch statusCode {
	case StatusInitializedUnsealed, StatusSealed:
		return statusCode, nil
	default:
		return statusCode, remoteflight.WrapErrorWith(err, stderr)
	}
}

// WaitForStatus waits until the vault service status matches one or more allowed
// status codes. If the context has a duration we will keep trying until it is done.
func WaitForStatus(ctx context.Context, ssh it.Transport, req *StatusRequest, statuses ...StatusCode) error {
	if len(statuses) == 0 {
		return nil
	}

	var err error
	status := StatusCode(-1)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for vault: %w: status: %s", ctx.Err(), statusToString(status))
		case <-ticker.C:
			status, err = Status(ctx, ssh, req)
			if err == nil {
				for _, s := range statuses {
					if status == s {
						return nil
					}
				}
			}
		}
	}
}

func statusToString(status StatusCode) string {
	switch status {
	case StatusInitializedUnsealed:
		return "initialized and unsealed"
	case StatusError:
		return "error"
	case StatusSealed:
		return "sealed"
	case StatusUnknown:
		return "unknown"
	default:
		return fmt.Sprint(status)
	}
}
