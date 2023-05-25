package consul

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// AgentHostRequest is a consul /v1/agent/host request.
type AgentHostRequest struct {
	FlightControlPath string
	ConsulAddr        string
}

// AgentHostRequest is a consul /v1/agent/host response.
type AgentHostResponse struct {
	Host *AgentHostResponseHost `json:"Host"`
}

// AgentHostResponseHost is the Host section of the response.
type AgentHostResponseHost struct {
	Hostname string `json:"hostname"`
	ID       string `json:"hostId"`
}

// AgentHostRequestOpt is a functional option agent host requests.
type AgentHostRequestOpt func(*AgentHostRequest) *AgentHostRequest

// NewAgentHostRequest takes functional options and returns a new request.
func NewAgentHostRequest(opts ...AgentHostRequestOpt) *AgentHostRequest {
	c := &AgentHostRequest{
		FlightControlPath: remoteflight.DefaultFlightControlPath,
		ConsulAddr:        "http://127.0.0.1:8500",
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithAgentHostRequestFlightControlPath sets the path to flightcontrol.
func WithAgentHostRequestFlightControlPath(path string) AgentHostRequestOpt {
	return func(u *AgentHostRequest) *AgentHostRequest {
		u.FlightControlPath = path
		return u
	}
}

// WithAgentHostRequestConsulAddr sets the consul bind address.
func WithAgentHostRequestConsulAddr(addr string) AgentHostRequestOpt {
	return func(u *AgentHostRequest) *AgentHostRequest {
		u.ConsulAddr = addr
		return u
	}
}

// GetAgentHost gets the agent host response.
func GetAgentHost(ctx context.Context, tr it.Transport, req *AgentHostRequest) (
	*AgentHostResponse,
	error,
) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var err error
	res := &AgentHostResponse{Host: &AgentHostResponseHost{}}

	if req.FlightControlPath == "" {
		err = errors.Join(err, fmt.Errorf("you must supply an enos-flight-control path"))
	}

	if req.ConsulAddr == "" {
		err = errors.Join(err, fmt.Errorf("you must supply a consul listen address"))
	}

	if err == nil {
		stdout, stderr, err1 := tr.Run(ctx, command.New(
			req.String(),
		))

		if err != nil {
			err = errors.Join(err, err1)
		}

		if stderr != "" {
			err = errors.Join(err, fmt.Errorf("unexpected write to STDERR: %s", stderr))
		}

		// Deserialize the body onto our response.
		if stdout == "" {
			err = errors.Join(err, fmt.Errorf("no JSON body was written to STDOUT"))
		} else {
			err = errors.Join(err, json.Unmarshal([]byte(stdout), res))
		}
	}

	if err != nil {
		return nil, errors.Join(fmt.Errorf("read /v1/agent/host"), err)
	}

	return res, nil
}

// String returns the request as a string.
func (r *AgentHostRequest) String() string {
	return fmt.Sprintf("%s download --url '%s/v1/agent/host' --stdout",
		r.FlightControlPath,
		r.ConsulAddr,
	)
}

// Hostname returns the hostname of the host.
func (r *AgentHostResponse) Hostname() string {
	if r.Host == nil {
		return ""
	}

	return r.Host.Hostname
}
