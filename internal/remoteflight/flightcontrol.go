// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package remoteflight

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/hashicorp/enos-provider/internal/flightcontrol"
	"github.com/hashicorp/enos-provider/internal/retry"
	"github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
)

// DefaultFlightControlPath is the default location of our binary.
const DefaultFlightControlPath = "/opt/qti/bin/enos-flight-control"

// InstallFlightControlRequest is a flight control install request.
type InstallFlightControlRequest struct {
	Path       string
	UseHomeDir bool
	*TargetRequest
}

// InstallFlightControlResponse is a flight control install response.
type InstallFlightControlResponse struct {
	Path string
}

// InstallFlightControlOpt is a functional option for an install request.
type InstallFlightControlOpt func(*InstallFlightControlRequest) *InstallFlightControlRequest

// NewInstallFlightControlRequest takes functional options and returns a new install request.
func NewInstallFlightControlRequest(opts ...InstallFlightControlOpt) *InstallFlightControlRequest {
	ir := &InstallFlightControlRequest{
		Path:          DefaultFlightControlPath,
		TargetRequest: NewTargetRequest(),
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

// WithInstallFlightControlRequestUseHomeDir installs enos-flight-control into the home
// directory.
func WithInstallFlightControlRequestUseHomeDir() InstallFlightControlOpt {
	return func(ir *InstallFlightControlRequest) *InstallFlightControlRequest {
		ir.UseHomeDir = true
		return ir
	}
}

// WithInstallFlightControlRequestTargetRequest sets the target request.
func WithInstallFlightControlRequestTargetRequest(tr *TargetRequest) InstallFlightControlOpt {
	return func(ir *InstallFlightControlRequest) *InstallFlightControlRequest {
		ir.TargetRequest = tr
		return ir
	}
}

// InstallFlightControl installs the enos-flight-control binary on a remote host in an
// idempotent fashion.
func InstallFlightControl(ctx context.Context, tr transport.Transport, ir *InstallFlightControlRequest) (*InstallFlightControlResponse, error) {
	res := &InstallFlightControlResponse{}

	select {
	case <-ctx.Done():
		return res, ctx.Err()
	default:
	}

	// Get the platform and architecture of the remote machine so that we can
	// make sure it's a supported target and so we can install the correct binary.
	platform, err := TargetPlatform(ctx, tr, ir.TargetRequest)
	if err != nil {
		return res, fmt.Errorf("determining target host platform: %w", err)
	}

	arch, err := TargetArchitecture(ctx, tr, ir.TargetRequest)
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

	path := ir.Path
	if ir.UseHomeDir {
		homeDir, err := TargetHomeDir(ctx, tr, NewTargetRequest())
		if err != nil {
			return res, fmt.Errorf("getting home directory to install enos-flight-control: %w", err)
		}

		path = filepath.Join(homeDir, "enos-flight-control")
		res.Path = path
	}

	// Check to see if we've already installed the binary
	_, _, err = tr.Run(ctx, command.New(fmt.Sprintf(`test -f '%s'`, path)))
	if err == nil {
		// enos-flight-control is already installed. If it's the latest version move on.
		var sha string

		// Try openssl since it would be there if we're using ssh.
		digestOut, _, err := tr.Run(ctx, command.New(fmt.Sprintf(`openssl dgst -sha256 '%s'`, path)))
		if err == nil {
			// Parse the SHA256 out of the command response
			// It looks something like "SHA256(enos-flight-control)= f43e9bc8c8f60bd067bafc5d59167184b14bd5dd57a17248e8a09e32bb42f515"
			trim := regexp.MustCompile(`^.* `)
			sha = trim.ReplaceAllLiteralString(digestOut, "")
		} else {
			// Try sha256sum
			sha, _, err = tr.Run(ctx, command.New(fmt.Sprintf(`sha256sum '%s' | cut -d ' ' -f1`, path)))
		}

		if err == nil && sha != "" {
			// One of our methods worked, check it against our binaries checksum
			desiredSha256 := fmt.Sprintf("%x", sha256.Sum256(flightControl))
			if sha == desiredSha256 {
				return res, nil
			}
		}
	}

	// Install the binary
	err = CopyFile(ctx, tr, NewCopyFileRequest(
		WithCopyFileContent(tfile.NewReader(string(flightControl))),
		WithCopyFileDestination(path),
		WithCopyFileChmod("+x"),
		WithCopyFileRetryOptions(
			retry.WithMaxRetries(ir.Retrier.MaxRetries),
			retry.WithIntervalFunc(ir.Retrier.RetryInterval),
			retry.WithOnlyRetryErrors(ir.Retrier.OnlyRetryError...),
		),
	))
	if err != nil {
		return res, fmt.Errorf("copying binary to target host: %w", err)
	}

	return res, nil
}
