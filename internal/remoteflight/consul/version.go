package consul

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// Version takes a context, transport, and path to the consul binary on a
// remote machine and returns the version.
func Version(ctx context.Context, tr it.Transport, req *CLIRequest) (semver.Version, error) {
	var ver semver.Version
	cmd := fmt.Sprintf("sudo %s version", req.BinPath)

	stdout, stderr, err := tr.Run(ctx, command.New(cmd))
	if err != nil {
		return ver, remoteflight.WrapErrorWith(err, stdout, stderr)
	}

	return parseConsulVersion(stdout)
}

// parseConsulVersion takes the consul version string and parses it.
func parseConsulVersion(version string) (semver.Version, error) {
	var ver semver.Version
	parts := strings.Split(version, " ")
	if len(parts) < 2 {
		return ver, fmt.Errorf("failed to parse version from consul version %s", version)
	}

	ver, err := semver.Make(strings.TrimLeft(parts[1], "v"))
	if err != nil {
		return ver, fmt.Errorf("failed to parse version from consul version %s", parts[1])
	}

	return ver, nil
}
