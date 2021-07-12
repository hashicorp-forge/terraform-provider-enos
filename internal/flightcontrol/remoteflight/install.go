package remoteflight

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"

	"github.com/hashicorp/enos-provider/internal/flightcontrol"
	"github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
)

// DefaultPath is the default location of our binary
const DefaultPath = "/opt/qti/bin/enos-flight-control"

// InstallRequest is a flight control install request
type InstallRequest struct {
	Path string
}

// InstallResponse is a flight control install response
type InstallResponse struct{}

// InstallOpt is a functional option for an install request
type InstallOpt func(*InstallRequest) *InstallRequest

// NewInstallRequest takes functional options and returns a new install request
func NewInstallRequest(opts ...InstallOpt) *InstallRequest {
	ir := &InstallRequest{
		Path: DefaultPath,
	}

	for _, opt := range opts {
		ir = opt(ir)
	}

	return ir
}

// WithInstallRequestPath sets the install path
func WithInstallRequestPath(path string) InstallOpt {
	return func(ir *InstallRequest) *InstallRequest {
		ir.Path = path
		return ir
	}
}

// Install installs the enos-flight-control binary on a remote host in an
// idempotent fashion.
func Install(ctx context.Context, ssh transport.Transport, ir *InstallRequest) (*InstallResponse, error) {
	res := &InstallResponse{}

	select {
	case <-ctx.Done():
		return res, ctx.Err()
	default:
	}

	// Get the platform and architecture of the remote machine so that we can
	// make sure it's a supported target and so we can install the correct binary.
	platform, err := TargetPlatform(ctx, ssh)
	if err != nil {
		return res, fmt.Errorf("determining target host platform: %w", err)
	}

	arch, err := TargetArchitecture(ctx, ssh)
	if err != nil {
		return res, fmt.Errorf("determining target host architecture: %w", err)
	}

	supported, err := flightcontrol.SupportedTarget(platform, arch)
	if err != nil {
		return res, fmt.Errorf("determining if target host is a supported platform and architecture: %w", err)
	}

	if !supported {
		return res, fmt.Errorf("install error: %s_%s is not a supported target", platform, arch)
	}

	flightControl, err := flightcontrol.ReadTargetFile(platform, arch)
	if err != nil {
		return res, fmt.Errorf("reading embedded enos-flight-control binary: %w", err)
	}

	// Check to see if we've already installed the binary
	_, _, err = ssh.Run(ctx, command.New(fmt.Sprintf(`test -f '%s'`, ir.Path)))
	if err == nil {
		// Something is installed in the path but we might need to update it.
		// We'll compare the SHA256 of the file installed with the version that
		// we intend to install. We use openssl to get the digest since we're
		// connecting with SSH and can reasonably assume that it's there.
		digestOut, _, err := ssh.Run(ctx, command.New(fmt.Sprintf(`openssl dgst -sha256 '%s'`, ir.Path)))
		if err == nil {
			// Parse the SHA256 out of the command response
			// It looks something like "SHA256(enos-flight-control)= f43e9bc8c8f60bd067bafc5d59167184b14bd5dd57a17248e8a09e32bb42f515"
			trim := regexp.MustCompile(`^.* `)
			installedSha256 := trim.ReplaceAllLiteralString(digestOut, "")
			desiredSha256 := fmt.Sprintf("%x", sha256.Sum256(flightControl))
			if installedSha256 == desiredSha256 {
				return res, nil
			}
		}
	}

	// Install the binary
	err = CopyFile(ctx, ssh, NewCopyFileRequest(
		WithCopyFileContent(tfile.NewReader(string(flightControl))),
		WithCopyFileDestination(ir.Path),
		WithCopyFileChmod("+x"),
	))
	if err != nil {
		return res, fmt.Errorf("copying binary to target host: %w", err)
	}

	return res, nil
}
