// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package remoteflight

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/retry"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
)

// TargetRequest is a Target* request.
type TargetRequest struct {
	*retry.Retrier
	RetryOpts []retry.RetrierOpt
}

// Clone create a cloned copy of the target request.
func (t *TargetRequest) Clone() *TargetRequest {
	if t == nil {
		return nil
	}

	c := &TargetRequest{
		RetryOpts: t.RetryOpts,
	}
	if t.Retrier != nil {
		c.Retrier = t.Retrier.Clone()
	}

	return c
}

// TargetRequestOpt is a functional option for a new Target.
type TargetRequestOpt func(*TargetRequest)

// HostInfo represents information about the target host.
type HostInfo struct {
	Arch            *string
	Distro          *string
	DistroVersion   *string
	Hostname        *string
	Pid1            *string
	Platform        *string
	PlatformVersion *string
}

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

// TargetDistro is a helper that determines the targets distribution.
func TargetDistro(ctx context.Context, tr transport.Transport, req *TargetRequest) (string, error) {
	req.Retrier.Func = func(ctx context.Context) (any, error) {
		var errs error

		// Try /etc/os-release
		content, stderr, err := tr.Run(ctx, command.New("cat /etc/os-release"))
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("attempting to get distro from /etc/os-release: %w: STDERR: %s", err, stderr))
		}
		if err == nil && content != "" {
			distro, err := findInOsRelease(content, "ID")
			errs = errors.Join(errs, fmt.Errorf("attempting to get distro from /etc/os-release content: %w: STDERR: %s", err, stderr))
			if distro != "" && err == nil {
				return distro, nil
			}
		}

		// Try lsb_release
		content, stderr, err = tr.Run(ctx, command.New("lsb_release -si"))
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("attempting to get distro from lsb_release: %w: STDERR: %s", err, stderr))
		}
		if err == nil && content != "" {
			return strings.ToLower(content), nil
		}

		return "", errs
	}

	distro, err := retry.Retry(ctx, req.Retrier)

	return distro.(string), err
}

// TargetDistroVersion is a helper that determines the targets distribution version.
func TargetDistroVersion(ctx context.Context, tr transport.Transport, req *TargetRequest) (string, error) {
	req.Retrier.Func = func(ctx context.Context) (any, error) {
		var errs error

		// Try /etc/os-release
		content, stderr, err := tr.Run(ctx, command.New("cat /etc/os-release"))
		if err != nil {
			errs = errors.Join(err, fmt.Errorf("attempting to get distro version from /etc/os-release: %w: STDERR: %s", err, stderr))
		}
		if err == nil && content != "" {
			version, err := findInOsRelease(content, "VERSION_ID")
			errs = errors.Join(err, fmt.Errorf("attempting to get distro version from /etc/os-release content: %w: STDERR: %s", err, stderr))
			if version != "" && err == nil {
				return version, nil
			}
		}

		// Try lsb_release
		content, stderr, err = tr.Run(ctx, command.New("lsb_release -sr"))
		if err != nil {
			errs = errors.Join(err, fmt.Errorf("attempting to get distro version from lsb_release: %w: STDERR: %s", err, stderr))
		}
		if err == nil && content != "" {
			return strings.ToLower(content), nil
		}

		return "", errs
	}

	version, err := retry.Retry(ctx, req.Retrier)

	return version.(string), err
}

// TargetHomeDir is a helper that determines the targets HOME directory.
func TargetHomeDir(ctx context.Context, tr transport.Transport, req *TargetRequest) (string, error) {
	req.Retrier.Func = func(ctx context.Context) (any, error) {
		var err error

		// Try the env variable
		home, stderr, err1 := tr.Run(ctx, command.New("echo $HOME"))
		if err1 != nil {
			err = errors.Join(err, fmt.Errorf("getting target home directory with $HOME env var: %w: STDERR: %s", err1, stderr))
		}
		if err1 == nil && home != "" {
			return home, nil
		}

		// Try tilde expansion
		home, stderr, err1 = tr.Run(ctx, command.New("echo ~"))
		if err1 != nil {
			err = errors.Join(err, fmt.Errorf("getting target home directory with ~ expansion: %w: STDERR: %s", err1, stderr))
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
				err = errors.Join(err, fmt.Errorf("getting target user %s from /etc/password: %w: STDERR: %s", me, err2, stderr))
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

// TargetHostInfo is a helper that determines the targets host information. Any errors will be returned.
// NOTE: not all platforms support all host info so handle the result and error accordingly.
func TargetHostInfo(ctx context.Context, tr transport.Transport, req *TargetRequest) (*HostInfo, error) {
	var errs error
	info := &HostInfo{}

	arch, err := TargetArchitecture(ctx, tr, req.Clone())
	errs = errors.Join(errs, err)
	info.Arch = &arch

	distro, err := TargetDistro(ctx, tr, req.Clone())
	errs = errors.Join(errs, err)
	info.Distro = &distro

	distroVer, err := TargetDistroVersion(ctx, tr, req.Clone())
	errs = errors.Join(errs, err)
	info.DistroVersion = &distroVer

	hostname, err := TargetHostname(ctx, tr, req.Clone())
	errs = errors.Join(errs, err)
	info.Hostname = &hostname

	pid1, err := TargetProcessManager(ctx, tr, req.Clone())
	errs = errors.Join(errs, err)
	info.Pid1 = &pid1

	platform, err := TargetPlatform(ctx, tr, req.Clone())
	errs = errors.Join(errs, err)
	info.Platform = &platform

	platformVer, err := TargetPlatformVersion(ctx, tr, req.Clone())
	errs = errors.Join(errs, err)
	info.PlatformVersion = &platformVer

	return info, errs
}

// TargetHostname is a helper that determines the targets hostname.
func TargetHostname(ctx context.Context, tr transport.Transport, req *TargetRequest) (string, error) {
	req.Retrier.Func = func(ctx context.Context) (any, error) {
		hostname, stderr, err := tr.Run(ctx, command.New("uname -n"))
		if err != nil {
			return "", fmt.Errorf("determining target hostname: %w, STDERR: %s", err, stderr)
		}

		if hostname == "" {
			return "", fmt.Errorf("failed to determine hostname, STDERR: %s", stderr)
		}

		return hostname, nil
	}

	hostname, err := retry.Retry(ctx, req.Retrier)
	if err != nil {
		return "", err
	}

	return hostname.(string), nil
}

// TargetPlatform is a helper that determines the targets platform.
func TargetPlatform(ctx context.Context, tr transport.Transport, req *TargetRequest) (string, error) {
	req.Retrier.Func = func(ctx context.Context) (any, error) {
		platform, stderr, err := tr.Run(ctx, command.New("uname -s"))
		if err != nil {
			return "", fmt.Errorf("determining target host platform: %w: STDERR: %s", err, stderr)
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

// TargetPlatformVersion is a helper that determines the targets platform version.
func TargetPlatformVersion(ctx context.Context, tr transport.Transport, req *TargetRequest) (string, error) {
	req.Retrier.Func = func(ctx context.Context) (any, error) {
		version, stderr, err := tr.Run(ctx, command.New("uname -r"))
		if err != nil {
			return "", fmt.Errorf("determining target host platform version: %w: STDERR: %s", err, stderr)
		}

		if version == "" {
			return "", fmt.Errorf("failed to determine platform version: STDERR: %s", stderr)
		}

		return version, nil
	}

	version, err := retry.Retry(ctx, req.Retrier)
	if err != nil {
		return "", err
	}

	return version.(string), nil
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
				return "", fmt.Errorf("failed to determine target process manager: %w: STDERR: %s", err, stderr)
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

func findInOsRelease(haystack string, needle string) (string, error) {
	if haystack == "" {
		return "", fmt.Errorf("cannot find %s in blank /etc/os-release", needle)
	}

	scanner := bufio.NewScanner(strings.NewReader(haystack))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "=")
		if len(parts) < 2 {
			continue
		}

		if parts[0] == needle {
			return strings.Trim(parts[1], `"`), nil
		}
	}
	err := fmt.Errorf("cannot find %s in /etc/os-release, /etc/os-release: %s", needle, haystack)
	if serr := scanner.Err(); serr != nil {
		err = errors.Join(serr, err)
	}

	return "", err
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
