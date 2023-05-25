package consul

import (
	"context"
	"fmt"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// ValidateFileRequest  is a Consul Config Validation request.
type ValidateFileRequest struct {
	*CLIRequest
	FilePath string
}

// ValidateFileRequestOpt is a functional option for a Consul validate command.
type ValidateFileRequestOpt func(*ValidateFileRequest) *ValidateFileRequest

// NewValidateFileRequest takes functional options and returns a new
// config validate request.
func NewValidateFileRequest(opts ...ValidateFileRequestOpt) *ValidateFileRequest {
	s := &ValidateFileRequest{
		CLIRequest: &CLIRequest{},
	}

	for _, opt := range opts {
		s = opt(s)
	}

	return s
}

// WithValidateConfigBinPath sets the Consul binary path.
func WithValidateConfigBinPath(path string) ValidateFileRequestOpt {
	return func(u *ValidateFileRequest) *ValidateFileRequest {
		u.BinPath = path
		return u
	}
}

// WithValidateFilePath sets the Consul config path.
func WithValidateFilePath(path string) ValidateFileRequestOpt {
	return func(u *ValidateFileRequest) *ValidateFileRequest {
		u.FilePath = path
		return u
	}
}

// ValidateConsulConfig validates the consul config using the consul validate command.
func ValidateConsulConfig(ctx context.Context, tr it.Transport, req *ValidateFileRequest) error {
	_, stderr, err := tr.Run(ctx, command.New(
		fmt.Sprintf("%s validate %s", req.BinPath, req.FilePath),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, stderr)
	}

	return nil
}

// ValidateConsulLicense validates the consul license using file path or env variable.
func ValidateConsulLicense(ctx context.Context, tr it.Transport, req *ValidateFileRequest) error {
	if req.FilePath == "" {
		return fmt.Errorf("you must provide a license file path ")
	}

	_, stderr, err := tr.Run(ctx, command.New(
		fmt.Sprintf("%s license inspect %s", req.BinPath, req.FilePath),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, stderr)
	}

	return nil
}
