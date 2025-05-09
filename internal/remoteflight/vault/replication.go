// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	istrings "github.com/hashicorp-forge/terraform-provider-enos/internal/strings"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
)

// ReplicationRequest is a replication request.
type ReplicationRequest struct {
	*CLIRequest
}

// ReplicationStatusResponse is the JSON stdout result /v1/sys/replication/status.
type ReplicationResponse struct {
	Data *ReplicationData `json:"data,omitempty"`
}

// ReplicationData is the replication response data.
type ReplicationData struct {
	DR          *ReplicationDataStatus `json:"dr,omitempty"`
	Performance *ReplicationDataStatus `json:"performance,omitempty"`
}

// ReplicationDataStatus is the replication status information.
type ReplicationDataStatus struct {
	ClusterID        string                  `json:"cluster_id,omitempty"`
	KnownSecondaries []string                `json:"known_secondaries,omitempty"`
	LastWAL          json.Number             `json:"last_wal,omitempty"`
	MerkleRoot       string                  `json:"merkle_root,omitempty"`
	Mode             string                  `json:"mode,omitempty"`
	Secondaries      []*ReplicationSecondary `json:"secondaries,omitempty"`
}

// ReplicationSecondary is the replication secondary data.
type ReplicationSecondary struct {
	APIAddress       string `json:"api_address,omitempty"`
	ClusterAddress   string `json:"cluster_address,omitempty"`
	ConnectionStatus string `json:"connection_status,omitempty"`
	LastHeartbeat    string `json:"last_heartbeat,omitempty"`
	NodeID           string `json:"node_id,omitempty"`
}

// ReplicationRequestOpt is a replication request function option.
type ReplicationRequestOpt func(*ReplicationRequest) *ReplicationRequest

// NewReplicationRequest takes functional options and returns a new replication
// request.
func NewReplicationRequest(opts ...ReplicationRequestOpt) *ReplicationRequest {
	c := &ReplicationRequest{
		CLIRequest: &CLIRequest{},
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithReplicationRequestBinPath sets the vault binary path.
func WithReplicationRequestBinPath(path string) ReplicationRequestOpt {
	return func(u *ReplicationRequest) *ReplicationRequest {
		u.BinPath = path
		return u
	}
}

// WithReplicationRequestVaultAddr sets the vault address.
func WithReplicationRequestVaultAddr(addr string) ReplicationRequestOpt {
	return func(u *ReplicationRequest) *ReplicationRequest {
		u.VaultAddr = addr
		return u
	}
}

// GetReplicationStatus returns the vault node status.
func GetReplicationStatus(ctx context.Context, tr it.Transport, req *ReplicationRequest) (*ReplicationResponse, error) {
	var err error
	res := NewReplicationResponse()

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

	if err == nil {
		stdout, stderr, err1 := tr.Run(ctx, command.New(
			req.BinPath+" read -format=json sys/replication/status",
			command.WithEnvVar("VAULT_ADDR", req.VaultAddr),
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
		return nil, errors.Join(err, errors.New("get vault replication status: vault read sys/replication/status"))
	}

	return res, nil
}

// String returns the ha status as a string.
func (s *ReplicationResponse) String() string {
	if s == nil || s.Data == nil {
		return ""
	}

	return s.Data.String()
}

// String returns the replication data as a string.
func (s *ReplicationData) String() string {
	if s == nil {
		return ""
	}

	var dr, perf string
	if s.DR != nil {
		dr = s.DR.String()
	}
	if s.Performance != nil {
		perf = s.Performance.String()
	}
	if dr == "" && perf == "" {
		return ""
	}

	out := new(strings.Builder)
	if dr != "" {
		_, _ = out.WriteString("DR\n" + istrings.Indent("  ", dr))
	}
	if perf != "" {
		_, _ = out.WriteString("Performance\n" + istrings.Indent("  ", perf))
	}

	return out.String()
}

// String returns the status information as a string.
func (s *ReplicationDataStatus) String() string {
	if s == nil {
		return ""
	}

	out := new(strings.Builder)
	_, _ = fmt.Fprintf(out, "Cluster ID: %s\n", s.ClusterID)
	if len(s.KnownSecondaries) > 0 {
		_, _ = fmt.Fprintln(out, "Known Secondaries")
		for i := range s.KnownSecondaries {
			_, _ = fmt.Fprintf(out, "  %s\n", s.KnownSecondaries[i])
		}
	}
	_, _ = fmt.Fprintf(out, "Last WAL: %s\n", s.LastWAL)
	_, _ = fmt.Fprintf(out, "Merkle Root: %s\n", s.MerkleRoot)
	_, _ = fmt.Fprintf(out, "Mode: %s\n", s.Mode)
	if len(s.Secondaries) > 0 {
		_, _ = fmt.Fprintln(out, "Secondaries")
		for i := range s.Secondaries {
			_, _ = out.WriteString(istrings.Indent("  ", s.Secondaries[i].String()))
		}
	}

	return out.String()
}

// String returns the seal data as a string.
func (s *ReplicationSecondary) String() string {
	if s == nil {
		return ""
	}

	out := new(strings.Builder)
	_, _ = fmt.Fprintln(out, "Secondary")
	_, _ = fmt.Fprintf(out, "  API Address: %s\n", s.APIAddress)
	_, _ = fmt.Fprintf(out, "  Cluster Address: %s\n", s.ClusterAddress)
	_, _ = fmt.Fprintf(out, "  Connection Status: %s\n", s.ConnectionStatus)
	_, _ = fmt.Fprintf(out, "  Last Heartbeat: %s\n", s.LastHeartbeat)
	_, _ = fmt.Fprintf(out, "  Node ID: %s\n", s.NodeID)

	return out.String()
}

// NewReplicationResponse returns a new instance of ReplicationResponse.
func NewReplicationResponse() *ReplicationResponse {
	return &ReplicationResponse{
		Data: &ReplicationData{
			DR:          NewReplicationDataStatus(),
			Performance: NewReplicationDataStatus(),
		},
	}
}

// NewReplicationDataStatus returns a new instance of ReplicationDataStatus.
func NewReplicationDataStatus() *ReplicationDataStatus {
	return &ReplicationDataStatus{Secondaries: []*ReplicationSecondary{}}
}
