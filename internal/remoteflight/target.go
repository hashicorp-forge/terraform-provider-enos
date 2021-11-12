package remoteflight

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/enos-provider/internal/retry"
	"github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// TargetPlatform is a helper that determines the targets platform
func TargetPlatform(ctx context.Context, ssh transport.Transport) (string, error) {
	// Get the platform and architecture of the remote machine so that we can
	// make sure it's a supported target and so we can install the correct binary.
	getPlatform := func(ctx context.Context) (interface{}, error) {
		platform, _, err := ssh.Run(ctx, command.New("uname -s"))
		if err != nil {
			return "", fmt.Errorf("determining target host platform: %w", err)
		}

		if platform == "" {
			return "", fmt.Errorf("failed to determine platform")
		}

		return platform, nil
	}

	r, err := retry.NewRetrier(
		retry.WithMaxRetries(5),
		retry.WithIntervalFunc(retry.IntervalFibonacci(time.Second)),
		retry.WithRetrierFunc(getPlatform),
	)
	if err != nil {
		return "", err
	}

	platform, err := r.Run(ctx)
	if err != nil {
		return "", err
	}

	return formatPlatform(platform.(string)), nil
}

// TargetArchitecture is a helper that determines the targets architecture
func TargetArchitecture(ctx context.Context, ssh transport.Transport) (string, error) {
	getArchitecture := func(ctx context.Context) (interface{}, error) {
		arch, _, err := ssh.Run(ctx, command.New("uname -m"))
		if err != nil {
			return "", fmt.Errorf("determining target host architecture: %w", err)
		}

		if arch == "" {
			return "", fmt.Errorf("failed to determine architecture")
		}

		return arch, nil
	}

	r, err := retry.NewRetrier(
		retry.WithMaxRetries(5),
		retry.WithIntervalFunc(retry.IntervalFibonacci(time.Second)),
		retry.WithRetrierFunc(getArchitecture),
	)
	if err != nil {
		return "", err
	}

	arch, err := r.Run(ctx)
	if err != nil {
		return "", err
	}

	return formatArch(arch.(string)), nil
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
