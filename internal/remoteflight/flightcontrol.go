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

// DefaultFlightControlPath is the default location of our binary.
const DefaultFlightControlPath = "/opt/qti/bin/enos-flight-control"

// InstallFlightControlRequest is a flight control install request.
type InstallFlightControlRequest struct {
	Path string
}

// InstallFlightControlResponse is a flight control install response.
type InstallFlightControlResponse struct{}

// InstallFlightControlOpt is a functional option for an install request.
type InstallFlightControlOpt func(*InstallFlightControlRequest) *InstallFlightControlRequest

// NewInstallFlightControlRequest takes functional options and returns a new install request.
func NewInstallFlightControlRequest(opts ...InstallFlightControlOpt) *InstallFlightControlRequest {
	ir := &InstallFlightControlRequest{
		Path: DefaultFlightControlPath,
	}

	for _, opt := range opts {
		ir = opt(ir)
	}

	return ir
}

// WithInstallFlightControlRequestPath sets the install path.
func WithInstallFlightControlRequestPath(path string) InstallFlightControlOpt {
	return func(ir *InstallFlightControlRequest) *InstallFlightControlRequest {
		ir.Path = path
		return ir
	}
}

// InstallFlightControl installs the enos-flight-control binary on a remote host in an
// idempotent fashion.
func InstallFlightControl(ctx context.Context, client transport.Transport, ir *InstallFlightControlRequest) (*InstallFlightControlResponse, error) {
	res := &InstallFlightControlResponse{}

	select {
	case <-ctx.Done():
		return res, ctx.Err()
	default:
	}

	// Get the platform and architecture of the remote machine so that we can
	// make sure it's a supported target and so we can install the correct binary.
	platform, err := TargetPlatform(ctx, client)
	if err != nil {
		return res, fmt.Errorf("determining target host platform: %w", err)
	}

	arch, err := TargetArchitecture(ctx, client)
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
	_, _, err = client.Run(ctx, command.New(fmt.Sprintf(`test -f '%s'`, ir.Path)))
	if err == nil {
		// Something is installed in the path but we might need to update it.
		// We'll compare the SHA256 of the file installed with the version that
		// we intend to install. We use openssl to get the digest since we're
		// connecting with SSH and can reasonably assume that it's there.
		digestOut, _, err := client.Run(ctx, command.New(fmt.Sprintf(`openssl dgst -sha256 '%s'`, ir.Path)))
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
	err = CopyFile(ctx, client, NewCopyFileRequest(
		WithCopyFileContent(tfile.NewReader(string(flightControl))),
		WithCopyFileDestination(ir.Path),
		WithCopyFileChmod("+x"),
	))
	if err != nil {
		return res, fmt.Errorf("copying binary to target host: %w", err)
	}

	return res, nil
}
