package vault

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// SetLegacyLicenseRequest is the legacy license set request
type SetLegacyLicenseRequest struct {
	*CLIRequest
	LicensePath    string
	LicenseContent string
}

// SetLegacyLicenseRequestOpt is a functional option for a legacy license request
type SetLegacyLicenseRequestOpt func(*SetLegacyLicenseRequest) *SetLegacyLicenseRequest

// NewSetLegacyLicenseRequest takes functional options and returns a new
// systemd unit request
func NewSetLegacyLicenseRequest(opts ...SetLegacyLicenseRequestOpt) *SetLegacyLicenseRequest {
	s := &SetLegacyLicenseRequest{
		CLIRequest: &CLIRequest{},
	}

	for _, opt := range opts {
		s = opt(s)
	}

	return s
}

// WithSetLegacyLicenseRequestBinPath sets the vault binary path
func WithSetLegacyLicenseRequestBinPath(path string) SetLegacyLicenseRequestOpt {
	return func(u *SetLegacyLicenseRequest) *SetLegacyLicenseRequest {
		u.BinPath = path
		return u
	}
}

// WithSetLegacyLicenseRequestVaultAddr sets the vault address
func WithSetLegacyLicenseRequestVaultAddr(addr string) SetLegacyLicenseRequestOpt {
	return func(u *SetLegacyLicenseRequest) *SetLegacyLicenseRequest {
		u.VaultAddr = addr
		return u
	}
}

// WithSetLegacyLicenseRequestLicensePath sets the vault license path
func WithSetLegacyLicenseRequestLicensePath(path string) SetLegacyLicenseRequestOpt {
	return func(u *SetLegacyLicenseRequest) *SetLegacyLicenseRequest {
		u.LicensePath = path
		return u
	}
}

// WithSetLegacyLicenseRequestLicenseContent sets the vault license content
func WithSetLegacyLicenseRequestLicenseContent(content string) SetLegacyLicenseRequestOpt {
	return func(u *SetLegacyLicenseRequest) *SetLegacyLicenseRequest {
		u.LicenseContent = content
		return u
	}
}

// WithSetLegacyLicenseRequestToken sets the vault license token
func WithSetLegacyLicenseRequestToken(token string) SetLegacyLicenseRequestOpt {
	return func(u *SetLegacyLicenseRequest) *SetLegacyLicenseRequest {
		u.Token = token
		return u
	}
}

// SetLegacyLicense sets the vault license using the /sys/license endpoint.
func SetLegacyLicense(ctx context.Context, ssh it.Transport, req *SetLegacyLicenseRequest) error {
	if req.LicensePath == "" && req.LicenseContent == "" {
		return fmt.Errorf("you must provide a license path or content")
	}

	if req.LicensePath != "" {
		file, err := os.Open(req.LicensePath)
		if err != nil {
			return err
		}

		body, err := io.ReadAll(file)
		if err != nil {
			return err
		}
		req.LicenseContent = string(body)
	}

	_, stderr, err := ssh.Run(ctx, command.New(
		fmt.Sprintf("%s write /sys/license text='%s'", req.BinPath, req.LicenseContent),
		command.WithEnvVars(map[string]string{
			"VAULT_ADDR":  req.VaultAddr,
			"VAULT_TOKEN": req.Token,
		}),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, stderr)
	}

	return nil
}
