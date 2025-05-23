// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
)

// HAStatusResponse is the JSON stdout result of /v1/sys/ha-status.
type HAStatusResponse struct {
	Data *HAStatusData `json:"data,omitempty"`
}

// HAStatusData is the data section of the ha-status result.
type HAStatusData struct {
	Nodes []*HAStatusNode `json:"nodes,omitempty"`
}

// HAStatusNode is a node in the ha-status result.
type HAStatusNode struct {
	ActiveNode     bool   `json:"active_node,omitempty"`
	APIAddress     string `json:"api_address,omitempty"`
	ClusterAddress string `json:"cluster_address,omitempty"`
	Hostname       string `json:"hostname,omitempty"`
	LastEcho       string `json:"last_echo,omitempty"`
	Version        string `json:"version,omitempty"`
	// ENT only fields are below
	RedundancyZone string `json:"redundancy_zone,omitempty"`
	UpgradeVersion string `json:"upgrade_version,omitempty"`
}

// GetHAStatus returns the vault HA status.
func GetHAStatus(ctx context.Context, tr it.Transport, req *CLIRequest) (*HAStatusResponse, error) {
	var err error
	res := NewHAStatusResponse()

	select {
	case <-ctx.Done():
		err = ctx.Err()
	default:
	}

	if req.BinPath == "" {
		err = errors.Join(err, errors.New("you must supply a vault bin path"))
	}

	if req.VaultAddr == "" {
		err = errors.Join(err, errors.New("you must supply a vault listen address"))
	}

	if req.Token == "" {
		err = errors.Join(err, errors.New("you must supply a vault token for the /v1/sys/ha-status endpoint"))
	}

	if err == nil {
		stdout, stderr, err1 := tr.Run(ctx, command.New(
			req.BinPath+" read -format=json sys/ha-status",
			command.WithEnvVar("VAULT_ADDR", req.VaultAddr),
			command.WithEnvVar("VAULT_TOKEN", req.Token),
		))
		if err1 != nil {
			err = err1
		}
		if stderr != "" {
			err = errors.Join(err, fmt.Errorf("unexpected write to stderr: %s", stderr))
		}

		// Deserialize the body onto our response.
		if stdout == "" {
			err = errors.Join(err, errors.New("no body was written to stdout"))
		} else {
			err = errors.Join(err, json.Unmarshal([]byte(stdout), res))
		}
	}

	if err != nil {
		return nil, errors.Join(errors.New("get vault ha-status: vault read sys/ha-status"))
	}

	return res, nil
}

// String returns the ha status as a string.
func (s *HAStatusResponse) String() string {
	if s == nil || s.Data == nil {
		return ""
	}

	return s.Data.String()
}

// String returns the ha-status data as a string.
func (s *HAStatusData) String() string {
	if s == nil || s.Nodes == nil || len(s.Nodes) < 1 {
		return ""
	}

	out := new(strings.Builder)
	for i := range s.Nodes {
		_, _ = out.WriteString(s.Nodes[i].String())
	}

	return out.String()
}

// String returns the ha-status node data as a string.
func (s *HAStatusNode) String() string {
	if s == nil {
		return ""
	}

	out := new(strings.Builder)
	_, _ = fmt.Fprintln(out, "Node")
	_, _ = fmt.Fprintf(out, "  Active Node: %t\n", s.ActiveNode)
	_, _ = fmt.Fprintf(out, "  API Address: %s\n", s.APIAddress)
	_, _ = fmt.Fprintf(out, "  Cluster Address: %s\n", s.ClusterAddress)
	_, _ = fmt.Fprintf(out, "  Hostname: %s\n", s.Hostname)

	if s.LastEcho != "" {
		_, _ = fmt.Fprintf(out, "  Last Echo: %s\n", s.LastEcho)
	}
	if s.RedundancyZone != "" {
		_, _ = fmt.Fprintf(out, "  Redundancy Zone: %s\n", s.RedundancyZone)
	}
	if s.UpgradeVersion != "" {
		_, _ = fmt.Fprintf(out, "  Upgrade Version: %s\n", s.UpgradeVersion)
	}

	return out.String()
}

// NewHAStatusResponse returns a new instance of HAStatusResponse.
func NewHAStatusResponse() *HAStatusResponse {
	return &HAStatusResponse{Data: &HAStatusData{Nodes: []*HAStatusNode{}}}
}
