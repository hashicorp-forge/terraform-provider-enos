package remoteflight

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/enos-provider/internal/retry"
	"github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// TargetRequest is a Target* request.
type TargetRequest struct {
	*retry.Retrier
	RetryOpts []retry.RetrierOpt
}

// TargetRequestOpt is a functional option for a new Target.
type TargetRequestOpt func(*TargetRequest)

// NewTargetRequest takes optional arguments and returns a new instance of
// TargetRequest.
func NewTargetRequest(opts ...TargetRequestOpt) *TargetRequest {
	req := &TargetRequest{
		Retrier: &retry.Retrier{
			MaxRetries:     retry.MaxRetriesUnlimited,
			RetryInterval:  retry.IntervalExponential(2 * time.Second),
			OnlyRetryError: []error{},
		},
	}

	for _, opt := range opts {
		opt(req)
	}

	for _, opt := range req.RetryOpts {
		opt(req.Retrier)
	}

	return req
}

// WithTargetRequestRetryOpts allows the caller to define retry options.
func WithTargetRequestRetryOpts(opts ...retry.RetrierOpt) TargetRequestOpt {
	return func(req *TargetRequest) {
		req.RetryOpts = opts
	}
}

// TargetPlatform is a helper that determines the targets platform.
func TargetPlatform(ctx context.Context, tr transport.Transport, req *TargetRequest) (string, error) {
	// Get the platform and architecture of the remote machine so that we can
	// make sure it's a supported target and so we can install the correct binary.
	req.Retrier.Func = func(ctx context.Context) (any, error) {
		platform, stderr, err := tr.Run(ctx, command.New("uname -s"))
		if err != nil {
			return "", fmt.Errorf("determining target host platform: %w, STDERR: %s", err, stderr)
		}

		if platform == "" {
			return "", fmt.Errorf("failed to determine platform, STDERR: %s", stderr)
		}

		return platform, nil
	}

	platform, err := retry.Retry(ctx, req.Retrier)
	if err != nil {
		return "", err
	}

	return formatPlatform(platform.(string)), nil
}

// TargetArchitecture is a helper that determines the targets architecture.
func TargetArchitecture(ctx context.Context, tr transport.Transport, req *TargetRequest) (string, error) {
	req.Retrier.Func = func(ctx context.Context) (any, error) {
		arch, stderr, err := tr.Run(ctx, command.New("uname -m"))
		if err != nil {
			return "", fmt.Errorf("determining target host architecture: %w, STDERR:%s", err, stderr)
		}

		if arch == "" {
			return "", fmt.Errorf("failed to determine architecture, STDERR: %s", stderr)
		}

		return arch, nil
	}

	arch, err := retry.Retry(ctx, req.Retrier)
	if err != nil || arch == nil {
		return "", err
	}

	return formatArch(arch.(string)), nil
}

// TargetHomeDir is a helper that determines the targets HOME directory.
func TargetHomeDir(ctx context.Context, tr transport.Transport, req *TargetRequest) (string, error) {
	req.Retrier.Func = func(ctx context.Context) (any, error) {
		var err error

		// Try the env variable
		home, stderr, err1 := tr.Run(ctx, command.New("echo $HOME"))
		if err1 != nil {
			err = errors.Join(err, fmt.Errorf("getting target home directory with $HOME env var: %w, STDERR: %s", err1, stderr))
		}
		if err1 == nil && home != "" {
			return home, nil
		}

		// Try tilde expansion
		home, stderr, err1 = tr.Run(ctx, command.New("echo ~"))
		if err1 != nil {
			err = errors.Join(err, fmt.Errorf("getting target home directory with ~ expansion: %w, STDERR: %s", err1, stderr))
		}
		if err1 == nil && home != "" {
			return home, nil
		}

		// Try /etc/password
		me, stderr, err1 := tr.Run(ctx, command.New("whoami"))
		if err1 != nil {
			err = errors.Join(err, fmt.Errorf("getting target user with 'whoami': %w, STDERR: %s", err1, stderr))
		}
		if err1 != nil && me != "" {
			home, stderr, err2 := tr.Run(ctx, command.New(fmt.Sprintf("grep %s /etc/passwd | cut -d: -f 6", me)))
			if err2 != nil {
				err = errors.Join(err, fmt.Errorf("getting target user %s from /etc/password: %w, STDERR: %s", me, err2, stderr))
			}
			if err2 == nil && home != "" {
				return home, nil
			}
		}

		return "", err
	}

	home, err := retry.Retry(ctx, req.Retrier)
	if err != nil || home == nil {
		return "", err
	}

	return home.(string), nil
}

// TargetProcessManager is a helper that determines the targets process manager.
func TargetProcessManager(ctx context.Context, tp transport.Transport, req *TargetRequest) (string, error) {
	switch tp.Type() {
	case transport.TransportType("ssh"):
		// Assume that were hitting a machine that doesn't have busybox ps and
		// supports the p flag. We could theoretically use /proc/ps/stat for
		// linux machines that have unsable ps but this is okay for now.
		req.Retrier.Func = func(ctx context.Context) (any, error) {
			pid1, stderr, err := tp.Run(ctx, command.New("ps -p 1 -c -o command="))
			if err != nil {
				return "", fmt.Errorf("failed to determine target process manager: %w, STDERR: %s", err, stderr)
			}

			if pid1 == "" {
				return "", fmt.Errorf("failed to determine target process manager, STDERR: %s", err)
			}

			return pid1, nil
		}

		pid1, err := retry.Retry(ctx, req.Retrier)
		if err != nil || pid1 == nil {
			return "", err
		}

		return pid1.(string), nil
	case transport.TransportType("kubernetes"):
		// Containers can have all sorts of process managers. We could actually
		// get pid1 but it's probably a big log nasty setup chain that calls
		// the container entry point. So instead we'll manage the process via
		// the K8s controller API.
		return "kubernetes", nil
	case transport.TransportType("nomad"):
		// Same story with Nomad. We'll rely on the Nomad API to determine
		// status of the jobs.
		return "nomad", nil
	default:
		return "", fmt.Errorf("failed to determine target process manager: unsupported transport type: %s", tp.Type())
	}
}

func formatPlatform(platform string) string {
	return strings.ToLower(platform)
}

func formatArch(arch string) string {
	switch arch {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return arch
	}
}
