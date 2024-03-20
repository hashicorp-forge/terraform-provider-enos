// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hashicorp/enos-provider/internal/log"
	"github.com/hashicorp/enos-provider/internal/remoteflight"
	"github.com/hashicorp/enos-provider/internal/remoteflight/systemd"
	"github.com/hashicorp/enos-provider/internal/retry"
	istrings "github.com/hashicorp/enos-provider/internal/strings"
	it "github.com/hashicorp/enos-provider/internal/transport"
)

// State represents the state of a node in a consul cluster.
type State struct {
	*AgentHostResponse          // /v1/agent/host
	*HealthNodeResponse         // /v1/health/node/:node
	*HealthStatePassingResponse // /v1/health/state/passing
	*RaftConfigurationResponse  // /v1/operator/raft/configuration
	systemd.UnitProperties      // systemd unit properties for consul.service
}

// StateRequest is a consul status request.
type StateRequest struct {
	FlightControlPath       string // where enos-flight-control is installed
	FlightControlUseHomeDir bool   // install enos-flight-control into the $HOME directory
	SystemdUnitName         string // what the systemd unit name for the consul service is
	ConsulAddr              string // consul bind address
}

// StateRequestOpt is a functional option for a config create request.
type StateRequestOpt func(*StateRequest) *StateRequest

// CheckStater is a validate function that takes a state and checks that it
// has expected values.
type CheckStater func(s *State) error

// NewState returns a new consul cluster node state.
func NewState() *State {
	return &State{}
}

// NewStateRequest takes functional options and returns a new
// systemd unit request.
func NewStateRequest(opts ...StateRequestOpt) *StateRequest {
	c := &StateRequest{
		FlightControlPath: remoteflight.DefaultFlightControlPath,
		SystemdUnitName:   "consul",
		ConsulAddr:        "http://127.0.0.1:8500",
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithStateRequestFlightControlPath sets the enos-flight-control binary path.
func WithStateRequestFlightControlPath(path string) StateRequestOpt {
	return func(u *StateRequest) *StateRequest {
		u.FlightControlPath = path
		return u
	}
}

// WithStateRequestFlightControlUseHomeDir will use the $HOME directory for the enos-flight-control
// binary path.
func WithStateRequestFlightControlUseHomeDir() StateRequestOpt {
	return func(u *StateRequest) *StateRequest {
		u.FlightControlUseHomeDir = true
		return u
	}
}

// WithStateRequestSystemdUnitName sets the flightcontrol binary path.
func WithStateRequestSystemdUnitName(name string) StateRequestOpt {
	return func(u *StateRequest) *StateRequest {
		u.SystemdUnitName = name
		return u
	}
}

// WithStateRequestConsulAddr sets the consul bind address.
func WithStateRequestConsulAddr(addr string) StateRequestOpt {
	return func(u *StateRequest) *StateRequest {
		u.ConsulAddr = addr
		return u
	}
}

// GetState returns the consul cluster node state.
func GetState(ctx context.Context, tr it.Transport, req *StateRequest) (*State, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// We use flightcontrol to read data from the consul API
	var err error

	opts := []remoteflight.InstallFlightControlOpt{
		remoteflight.WithInstallFlightControlRequestTargetRequest(
			// Don't auto-retry installing enos-flight-control. WaitForState
			// can handle retrying for us.
			remoteflight.NewTargetRequest(
				remoteflight.WithTargetRequestRetryOpts(
					retry.WithMaxRetries(0),
				),
			),
		),
	}
	if req.FlightControlUseHomeDir {
		opts = append(opts, remoteflight.WithInstallFlightControlRequestUseHomeDir())
	} else {
		opts = append(opts, remoteflight.WithInstallFlightControlRequestPath(req.FlightControlPath))
	}

	// We use flightcontrol to read data from the vault /v1/sys/health API
	fcRes, err := remoteflight.InstallFlightControl(ctx, tr, remoteflight.NewInstallFlightControlRequest(opts...))
	if err != nil {
		return nil, fmt.Errorf("installing enos-flight-control binary to get consul state: %w", err)
	}

	state := NewState()

	// Get the systemd unit properties
	sysd := systemd.NewClient(tr, log.NewLogger(ctx))
	if err != nil {
		return nil, fmt.Errorf("getting the systemd unit properties: %w", err)
	}

	state.UnitProperties, err = sysd.ShowProperties(ctx, req.SystemdUnitName)
	if err != nil {
		return state, fmt.Errorf("getting the systemd unit properties: %w", err)
	}

	state.AgentHostResponse, err = GetAgentHost(
		ctx, tr, NewAgentHostRequest(
			WithAgentHostRequestConsulAddr(req.ConsulAddr),
			WithAgentHostRequestFlightControlPath(fcRes.Path),
		),
	)
	if err != nil {
		return state, err
	}

	state.HealthNodeResponse, err = GetHealthNode(
		ctx, tr, NewHealthNodeRequest(
			WithHealthNodeRequestFlightControlPath(fcRes.Path),
			WithHealthNodeRequestConsulAddr(req.ConsulAddr),
			WithHealthNodeRequestNodeName(state.AgentHostResponse.Hostname()),
		),
	)
	if err != nil {
		return state, err
	}

	state.HealthStatePassingResponse, err = GetHealthStatePassing(
		ctx, tr, NewHealthStatePassingRequest(
			WithHealthStatePassingRequestFlightControlPath(fcRes.Path),
			WithHealthStatePassingRequestConsulAddr(req.ConsulAddr),
		),
	)
	if err != nil {
		return state, err
	}

	state.RaftConfigurationResponse, err = GetRaftConfiguration(
		ctx, tr, NewRaftConfigurationRequest(
			WithRaftConfigurationRequestFlightControlPath(fcRes.Path),
			WithRaftConfigurationRequestConsulAddr(req.ConsulAddr),
		),
	)

	return state, err
}

// WaitForState waits until the consul cluster node state satisfies all of the
// provided checks.
func WaitForState(ctx context.Context, tr it.Transport, req *StateRequest, checks ...CheckStater) (*State, error) {
	checkState := func(ctx context.Context) (any, error) {
		var err error

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		state, err := GetState(ctx, tr, req)
		if err != nil {
			return state, err
		}

		if len(checks) == 0 {
			return state, nil
		}

		if state == nil {
			return nil, errors.Join(err, errors.New("no state data found"))
		}

		for _, check := range checks {
			err = check(state)
			if err != nil {
				return state, err
			}
		}

		return state, nil
	}

	r, err := retry.NewRetrier(
		retry.WithIntervalFunc(retry.IntervalDuration(5*time.Second)),
		retry.WithRetrierFunc(checkState),
	)

	var state any
	if err == nil {
		state, err = retry.Retry(ctx, r)
	}

	if err != nil {
		err = fmt.Errorf("waiting for consul to enter desired state: %w", err)
	}

	if state == nil {
		return nil, err
	}

	return state.(*State), err
}

// String returns our state as a string.
func (s *State) String() string {
	out := new(strings.Builder)

	_, _ = out.WriteString(fmt.Sprintf("Agent Hostname: %s\n", s.AgentHostResponse.Hostname()))
	s.printStateField(out, s.HealthNodeResponse, "Node Health")
	s.printStateField(out, s.HealthStatePassingResponse, "Healthy Nodes")
	s.printStateField(out, s.RaftConfigurationResponse, "Raft Configuration")

	// Most of the time we don't care about all of the systemd unit properties.
	// Try and find our meaningful status properties. If we can't then something
	// strange is afoot and we'll display all of our props.
	props, err := s.UnitProperties.FindProperties(systemd.EnabledAndRunningProperties)
	if err != nil {
		props = s.UnitProperties
	}
	s.printStateField(out, props, "Systemd Unit Properties")

	return out.String()
}

// printStateField takes a writer, a string, and field name and writes the
// body of the stringer result indented.
func (s *State) printStateField(w io.Writer, f fmt.Stringer, fn string) {
	if f == nil || w == nil {
		return
	}

	fs := f.String()
	if fs == "" {
		return
	}
	_, _ = w.Write([]byte(fmt.Sprintf("%s: \n%s\n", fn, istrings.Indent("  ", fs))))
}
