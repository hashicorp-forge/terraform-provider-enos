// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight"
	istrings "github.com/hashicorp-forge/terraform-provider-enos/internal/strings"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
)

// RaftConfigurationRequest is a consul /v1/operator/raft/configuration request.
type RaftConfigurationRequest struct {
	FlightControlPath string
	ConsulAddr        string
}

// RaftConfigurationResponse is a consul /v1/operator/raft/configuration response.
type RaftConfigurationResponse struct {
	Servers []*RaftServer `json:"Servers"`
}

// RaftServer is raft server.
type RaftServer struct {
	ID              string      `json:"ID"`
	Node            string      `json:"Node"`
	Address         string      `json:"Address"`
	Leader          bool        `json:"Leader"`
	ProtocolVersion json.Number `json:"ProtocolVersion"`
	Voter           bool        `json:"Voter"`
}

// RaftConfigurationRequestOpt is a functional option agent host requests.
type RaftConfigurationRequestOpt func(*RaftConfigurationRequest) *RaftConfigurationRequest

// NewRaftConfigurationRequest takes functional options and returns a new request.
func NewRaftConfigurationRequest(opts ...RaftConfigurationRequestOpt) *RaftConfigurationRequest {
	c := &RaftConfigurationRequest{
		FlightControlPath: remoteflight.DefaultFlightControlPath,
		ConsulAddr:        "http://127.0.0.1:8500",
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithRaftConfigurationRequestFlightControlPath sets the path to flightcontrol.
func WithRaftConfigurationRequestFlightControlPath(path string) RaftConfigurationRequestOpt {
	return func(u *RaftConfigurationRequest) *RaftConfigurationRequest {
		u.FlightControlPath = path
		return u
	}
}

// WithRaftConfigurationRequestConsulAddr sets the consul bind address.
func WithRaftConfigurationRequestConsulAddr(addr string) RaftConfigurationRequestOpt {
	return func(u *RaftConfigurationRequest) *RaftConfigurationRequest {
		u.ConsulAddr = addr
		return u
	}
}

// GetRaftConfiguration gets the agent host response.
func GetRaftConfiguration(ctx context.Context, tr it.Transport, req *RaftConfigurationRequest) (
	*RaftConfigurationResponse,
	error,
) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var err error
	res := &RaftConfigurationResponse{Servers: make([]*RaftServer, 0)}

	if req.FlightControlPath == "" {
		err = errors.Join(err, errors.New("you must supply an enos-flight-control path"))
	}

	if req.ConsulAddr == "" {
		err = errors.Join(err, errors.New("you must supply a consul listen address"))
	}

	if err == nil {
		stdout, stderr, err1 := tr.Run(ctx, command.New(
			req.String(),
		))

		if err1 != nil {
			err = err1
		}

		if stderr != "" {
			err = errors.Join(err, fmt.Errorf("unexpected write to STDERR: %s", stderr))
		}

		// Deserialize the body onto our response.
		if stdout == "" {
			err = errors.Join(err, errors.New("no JSON body was written to STDOUT"))
		} else {
			err = errors.Join(err, json.Unmarshal([]byte(stdout), res))
		}
	}

	if err != nil {
		return nil, errors.Join(errors.New("read /v1/operator/raft/configuruation"), err)
	}

	return res, nil
}

// String returns the request as a string.
func (r *RaftConfigurationRequest) String() string {
	return fmt.Sprintf("%s download --url '%s/v1/operator/raft/configuration' --stdout",
		r.FlightControlPath,
		r.ConsulAddr,
	)
}

// String returns the raft configuration response as a string.
func (r *RaftConfigurationResponse) String() string {
	if r == nil || r.Servers == nil || len(r.Servers) < 1 {
		return ""
	}

	out := new(strings.Builder)
	_, _ = fmt.Fprintln(out, "Servers")
	for i := range r.Servers {
		_, _ = out.WriteString(istrings.Indent("  ", r.Servers[i].String()))
	}

	return out.String()
}

// String returns the RaftServer as a string.
func (s *RaftServer) String() string {
	if s == nil {
		return ""
	}

	out := new(strings.Builder)
	fmt.Fprintf(out, "Node: %s\n", s.Node)
	fmt.Fprintf(out, "ID: %s\n", s.ID)
	fmt.Fprintf(out, "Address: %s\n", s.Address)
	fmt.Fprintf(out, "Leader: %t\n", s.Leader)
	fmt.Fprintf(out, "ProtocolVersion: %s\n", s.ProtocolVersion)
	fmt.Fprintf(out, "Voter: %t\n", s.Voter)

	return out.String()
}
