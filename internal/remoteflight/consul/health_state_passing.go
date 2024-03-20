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

// HealthStatePassingRequest is a consul /v1/health/state/passing request.
type HealthStatePassingRequest struct {
	FlightControlPath string
	ConsulAddr        string
}

// HealthStatePassingRequest is a consul /v1/health/state/passing response.
type HealthStatePassingResponse struct {
	Nodes []*NodeHealth `json:""`
}

// HealthStatePassingRequestOpt is a functional option agent host requests.
type HealthStatePassingRequestOpt func(*HealthStatePassingRequest) *HealthStatePassingRequest

// NewHealthStatePassingRequest takes functional options and returns a new request.
func NewHealthStatePassingRequest(opts ...HealthStatePassingRequestOpt) *HealthStatePassingRequest {
	c := &HealthStatePassingRequest{
		FlightControlPath: remoteflight.DefaultFlightControlPath,
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithHealthStatePassingRequestFlightControlPath sets the path to flightcontrol.
func WithHealthStatePassingRequestFlightControlPath(path string) HealthStatePassingRequestOpt {
	return func(u *HealthStatePassingRequest) *HealthStatePassingRequest {
		u.FlightControlPath = path
		return u
	}
}

// WithHealthStatePassingRequestConsulAddr sets the consul bind address.
func WithHealthStatePassingRequestConsulAddr(addr string) HealthStatePassingRequestOpt {
	return func(u *HealthStatePassingRequest) *HealthStatePassingRequest {
		u.ConsulAddr = addr
		return u
	}
}

// GetHealthStatePassing gets the agent host response.
func GetHealthStatePassing(ctx context.Context, tr it.Transport, req *HealthStatePassingRequest) (
	*HealthStatePassingResponse,
	error,
) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var err error
	res := &HealthStatePassingResponse{Nodes: make([]*NodeHealth, 0)}

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

		if err != nil {
			err = errors.Join(err, err1)
		}

		if stderr != "" {
			err = errors.Join(err, fmt.Errorf("unexpected write to STDERR: %s", stderr))
		}

		// Deserialize the body onto our response.
		if stdout == "" {
			err = errors.Join(err, errors.New("no JSON body was written to STDOUT"))
		} else {
			err = errors.Join(err, json.Unmarshal([]byte(stdout), &res.Nodes))
		}
	}

	if err != nil {
		return nil, errors.Join(errors.New("read /v1/health/state/passing"), err)
	}

	return res, nil
}

// String returns the request as a string.
func (r *HealthStatePassingRequest) String() string {
	return fmt.Sprintf("%s download --url '%s/v1/health/state/passing' --stdout",
		r.FlightControlPath,
		r.ConsulAddr,
	)
}

// String returns the NodeHealthResponse as a string.
func (n *HealthStatePassingResponse) String() string {
	if n == nil || n.Nodes == nil || len(n.Nodes) < 1 {
		return ""
	}

	out := new(strings.Builder)
	_, _ = out.WriteString("Healthy Nodes")

	for i := range n.Nodes {
		i := i
		_, _ = out.WriteString(istrings.Indent("  ", n.Nodes[i].String()))
	}

	return out.String()
}
