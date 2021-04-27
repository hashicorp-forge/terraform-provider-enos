package remoteflight

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// TargetPlatform is a helper that determines the targets platform
func TargetPlatform(ctx context.Context, ssh transport.Transport) (string, error) {
	// Get the platform and architecture of the remote machine so that we can
	// make sure it's a supported target and so we can install the correct binary.
	platform, _, err := ssh.Run(ctx, command.New("uname -s"))
	if err != nil {
		return "", fmt.Errorf("determining target host platform: %w", err)
	}

	return formatPlatform(platform), nil
}

// TargetArchitecture is a helper that determines the targets architecture
func TargetArchitecture(ctx context.Context, ssh transport.Transport) (string, error) {
	arch, _, err := ssh.Run(ctx, command.New("uname -m"))
	if err != nil {
		return "", fmt.Errorf("determining target host architecture: %w", err)
	}

	return formatArch(arch), nil
}

func formatPlatform(platform string) string {
	return strings.ToLower(platform)
}

func formatArch(arch string) string {
	switch arch {
	case "x86_64":
		return "amd64"
	default:
		return arch
	}
}
