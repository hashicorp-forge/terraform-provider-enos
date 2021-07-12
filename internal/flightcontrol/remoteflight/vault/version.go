package vault

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver"

	"github.com/hashicorp/enos-provider/internal/flightcontrol/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// Version takes a context, transport, and path to the vault binary on a
// remote machine and returns the version.
func Version(ctx context.Context, ssh it.Transport, req *StatusRequest) (semver.Version, error) {
	var ver semver.Version
	cmd := fmt.Sprintf("sudo %s version", req.BinPath)

	stdout, stderr, err := ssh.Run(ctx, command.New(cmd))
	if err != nil {
		return ver, remoteflight.WrapErrorWith(err, stdout, stderr)
	}

	return parseVaultVersion(stdout)
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
