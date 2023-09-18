package consul

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// HealthNodeRequest is a consul /v1/health/node/:node request.
type HealthNodeRequest struct {
	FlightControlPath string
	NodeName          string
	ConsulAddr        string
}

// HealthNodeResponse is a consul /v1/health/node/:node response.
type HealthNodeResponse struct {
	Nodes []*NodeHealth `json:""`
}

// NodeHealth is the Node section of the response.
type NodeHealth struct {
	Node   string `json:"Node"`
	Status string `json:"Status"`
	Output string `json:"Output"`
	Notes  string `json:"Notes"`
}

// NodeHealthStatusHealthy is the "healthy" status of a node.
const NodeHealthStatusHealthy = "passing"

// HealthNodeRequestOpt is a functional option agent host requests.
type HealthNodeRequestOpt func(*HealthNodeRequest) *HealthNodeRequest

// NewHealthNodeRequest takes functional options and returns a new request.
func NewHealthNodeRequest(opts ...HealthNodeRequestOpt) *HealthNodeRequest {
	c := &HealthNodeRequest{
		FlightControlPath: remoteflight.DefaultFlightControlPath,
		ConsulAddr:        "http://127.0.0.1:8500",
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithHealthNodeRequestFlightControlPath sets the path to flightcontrol.
func WithHealthNodeRequestFlightControlPath(path string) HealthNodeRequestOpt {
	return func(u *HealthNodeRequest) *HealthNodeRequest {
		u.FlightControlPath = path
		return u
	}
}

// WithHealthNodeRequestConsulAddr sets consul bind address.
func WithHealthNodeRequestConsulAddr(addr string) HealthNodeRequestOpt {
	return func(u *HealthNodeRequest) *HealthNodeRequest {
		u.ConsulAddr = addr
		return u
	}
}

// WithHealthNodeRequestNodeName sets the node name we're getting the health for.
func WithHealthNodeRequestNodeName(name string) HealthNodeRequestOpt {
	return func(u *HealthNodeRequest) *HealthNodeRequest {
		u.NodeName = name
		return u
	}
}

// GetHealthNode gets the agent host response.
func GetHealthNode(ctx context.Context, tr it.Transport, req *HealthNodeRequest) (
	*HealthNodeResponse,
	error,
) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var err error
	res := &HealthNodeResponse{Nodes: make([]*NodeHealth, 0)}

	if req.FlightControlPath == "" {
		err = errors.Join(err, fmt.Errorf("you must supply an enos-flight-control path"))
	}

	if req.ConsulAddr == "" {
		err = errors.Join(err, fmt.Errorf("you must supply a consul listen address"))
	}

	if req.NodeName == "" {
		err = errors.Join(err, fmt.Errorf("you must supply a node name"))
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
			err = errors.Join(err, json.Unmarshal([]byte(stdout), &res.Nodes))
		}
	}

	if err != nil {
		return nil, errors.Join(fmt.Errorf("read /v1/health/node"), err)
	}

	return res, nil
}

// String returns the request as a string.
func (r *HealthNodeRequest) String() string {
	return fmt.Sprintf("%s download --url '%s/v1/health/node/%s' --stdout",
		r.FlightControlPath,
		r.ConsulAddr,
		r.NodeName,
	)
}

// String returns the NodeHealthResponse as a string.
func (n *HealthNodeResponse) String() string {
	if n == nil || n.Nodes == nil || len(n.Nodes) < 1 {
		return ""
	}

	return n.Nodes[0].String()
}

// String returns the NodeHealth as a string.
func (n *NodeHealth) String() string {
	out := new(strings.Builder)

	if n.Node != "" {
		out.WriteString(fmt.Sprintf("Node: %s\n", n.Node))
	}

	if n.Status != "" {
		out.WriteString(fmt.Sprintf("Status: %s\n", n.Status))
	}

	if n.Output != "" {
		out.WriteString(fmt.Sprintf("Output: %s\n", n.Output))
	}

	if n.Notes != "" {
		out.WriteString(fmt.Sprintf("Notes: %s\n", n.Notes))
	}

	return out.String()
}
