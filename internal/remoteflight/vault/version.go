// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
)

// Version takes a context, transport, and path to the vault binary on a
// remote machine and returns the version.
func Version(ctx context.Context, tr it.Transport, req *StateRequest) (semver.Version, error) {
	var ver semver.Version
	cmd := fmt.Sprintf("sudo %s version", req.BinPath)

	stdout, stderr, err := tr.Run(ctx, command.New(cmd))
	if err != nil {
		return ver, remoteflight.WrapErrorWith(err, stdout, stderr)
	}

	return parseVaultVersion(strings.ReplaceAll(stdout, "\n", ""))
}

// parseVaultVersion takes the vault version string and parses it.
func parseVaultVersion(version string) (semver.Version, error) {
	var ver semver.Version
	parts := strings.Split(version, " ")
	if len(parts) < 2 {
		return ver, fmt.Errorf("failed to parse version from vault version %s", version)
	}

	ver, err := semver.Make(strings.TrimLeft(parts[1], "v"))
	if err != nil {
		return ver, fmt.Errorf("failed to parse version from vault version %s", parts[1])
	}

	return ver, nil
}
