package consul

import (
	"context"
	"errors"
	"fmt"
	"time"

	xssh "golang.org/x/crypto/ssh"

	"github.com/hashicorp/enos-provider/internal/flightcontrol/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// StatusCode are Consul status exit codes
type StatusCode int

// Consul status exit codes
const (
	StatusRunning StatusCode = 0
	StatusError   StatusCode = 1
	// StatusUnknown is returned if a non-consul status error code is encountered
	StatusUnknown StatusCode = 9
)

// StatusRequest is a consul status request
type StatusRequest struct {
	*CLIRequest
}

// StatusRequestOpt is a functional option for a config create request
type StatusRequestOpt func(*StatusRequest) *StatusRequest

// NewStatusRequest takes functional options and returns a new
// systemd unit request
func NewStatusRequest(opts ...StatusRequestOpt) *StatusRequest {
	c := &StatusRequest{
		&CLIRequest{},
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithStatusRequestBinPath sets the consul binary path
func WithStatusRequestBinPath(path string) StatusRequestOpt {
	return func(u *StatusRequest) *StatusRequest {
		u.BinPath = path
		return u
	}
}

// Status returns the consul status code
func Status(ctx context.Context, ssh it.Transport, req *StatusRequest) (StatusCode, error) {
	if req.BinPath == "" {
		return StatusUnknown, fmt.Errorf("you must supply a consul bin path")
	}

	_, stderr, err := ssh.Run(ctx, command.New(
		fmt.Sprintf("%s operator raft list-peers", req.BinPath),
	))
	// If we don't get an error consul is running
	if err == nil {
		return StatusRunning, nil
	}

	// Determine what the error status is and if we need to return an error to
	// the caller.
	statusCode := StatusUnknown
	var exitError *xssh.ExitError
	if errors.As(err, &exitError) {
		statusCode = StatusCode(exitError.Waitmsg.ExitStatus())
	}

	switch statusCode {
	case StatusRunning:
		return statusCode, nil
	default:
		return statusCode, remoteflight.WrapErrorWith(err, stderr)
	}
}

// WaitForStatus waits until the consul service status matches one or more allowed
// status codes. If the context has a duration we will keep trying until it is done.
func WaitForStatus(ctx context.Context, ssh it.Transport, req *StatusRequest, statuses ...StatusCode) error {
	if len(statuses) == 0 {
		return nil
	}

	var err error
	status := StatusCode(-1)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for consul: %w: status: %s", ctx.Err(), statusToString(status))
		case <-ticker.C:
			status, err = Status(ctx, ssh, req)
			if err == nil {
				for _, s := range statuses {
					if status == s {
						return nil
					}
				}
			}
		}
	}
}

func statusToString(status StatusCode) string {
	switch status {
	case StatusRunning:
		return "consul service is running"
	case StatusError:
		return "error"
	case StatusUnknown:
		return "unknown"
	default:
		return fmt.Sprint(status)
	}
}
