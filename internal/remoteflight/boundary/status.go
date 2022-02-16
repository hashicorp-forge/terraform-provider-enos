package boundary

import (
	"context"
	"errors"
	"fmt"
	"time"

	xssh "golang.org/x/crypto/ssh"

	"github.com/hashicorp/enos-provider/internal/retry"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// StatusCode is a systemd exit code from a status check
type StatusCode int

const (
	StatusActive   StatusCode = 0
	StatusInactive StatusCode = 3 // or, unfortunately, "activating", thanks systemd
	StatusUnknown  StatusCode = 9
)

// StatusRequest is a boundary status request
type StatusRequest struct {
	*CLIRequest
}

// StatusRequestOpt is a functional option for a status request
type StatusRequestOpt func(*StatusRequest)

// NewStatusRequest takes functional options and returns a new
// status request
func NewStatusRequest(opts ...StatusRequestOpt) *StatusRequest {
	c := &StatusRequest{
		&CLIRequest{},
	}

	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithStatusRequestBinPath sets the boundary binary path
func WithStatusRequestBinPath(path string) StatusRequestOpt {
	return func(u *StatusRequest) {
		u.BinPath = path
	}
}

// Status returns the systemd status of the boundary service (until we have a better way)
func Status(ctx context.Context, ssh it.Transport, unitName string) (StatusCode, error) {
	_, _, err := ssh.Run(ctx, command.New(
		fmt.Sprintf("systemctl is-active %s", unitName),
	))
	// if we return no err, service is active
	if err == nil {
		return StatusActive, nil
	}

	// otherwise, set status to Unknown by default and extract the code from xssh
	statusCode := StatusUnknown
	var exitError *xssh.ExitError
	if errors.As(err, &exitError) {
		statusCode = StatusCode(exitError.Waitmsg.ExitStatus())
	}
	return statusCode, nil
}

// WaitForService waits until the boundary service status is active according to systemd
func WaitForService(ctx context.Context, ssh it.Transport) error {
	var err error

	statusCheck := func(ctx context.Context) (interface{}, error) {
		var res interface{}
		res, err := Status(ctx, ssh, "boundary")
		if err != nil {
			return res, err
		}
		return res, err
	}

	r, err := retry.NewRetrier(
		retry.WithMaxRetries(10),
		retry.WithIntervalFunc(retry.IntervalFibonacci(time.Second)),
		retry.WithRetrierFunc(statusCheck),
	)
	if err != nil {
		return err
	}

	_, err = retry.Retry(ctx, r)
	if err != nil {
		return err
	}
	return nil
}
