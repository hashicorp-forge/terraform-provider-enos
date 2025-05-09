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

// RaftConfigurationResponse is the response of /v1/sys/raft/configuration.
type RaftConfigurationResponse struct {
	Data *RaftConfigurationData `json:"data,omitempty"`
}

// RaftConfigurationData is the data stanza of the raft response.
type RaftConfigurationData struct {
	Config *RaftConfigurationDataConfig `json:"config,omitempty"`
}

// RaftConfigurationData is the config stanza of the raft response.
type RaftConfigurationDataConfig struct {
	Index   json.Number                `json:"index,omitempty"`
	Servers []*RaftConfigurationServer `json:"servers,omitempty"`
}

// RaftConfigurationServer is one of the raft servers that have been configured.
type RaftConfigurationServer struct {
	Address         string `json:"address,omitempty"`
	Leader          bool   `json:"leader,omitempty"`
	NodeID          string `json:"node_id,omitempty"`
	ProtocolVersion string `json:"protocol_version,omitempty"`
	Voter           bool   `json:"voter,omitempty"`
}

// RaftAutopilotConfigurationResponse is the response of /v1/sys/raft/autopilot/configuration.
type RaftAutopilotConfigurationResponse struct {
	Data *RaftAutopilotConfigurationData `json:"data,omitempty"`
}

// RaftAutopilotConfigurationData is the data stanza of the config response.
type RaftAutopilotConfigurationData struct {
	CleanupDeadServers             bool        `json:"cleanup_dead_servers,omitempty"`
	DeadServerLastContactThreshold string      `json:"dead_server_last_contact_threshold,omitempty"`
	LastContactThreshold           string      `json:"last_contact_threshold,omitempty"`
	MaxTrailingLogs                json.Number `json:"max_trailing_logs,omitempty"`
	MinQuorum                      json.Number `json:"min_quorum,omitempty"`
	ServerStabilizationTime        string      `json:"server_stabilization_time,omitempty"`
	DisableUpgradeMigration        bool        `json:"disable_upgrade_migration,omitempty"`
}

// RaftAutopilotStateResponse is the raft autopilot state.
type RaftAutopilotStateResponse struct {
	Data *RaftAutopilotStateResponseData `json:"data,omitempty"`
}

// RaftAutopilotStateResponseData is the raft autopilot state data.
type RaftAutopilotStateResponseData struct {
	Healthy                    bool        `json:"healthy,omitempty"`
	FailureTolerance           json.Number `json:"failure_tolerance,omitempty"`
	Leader                     string      `json:"leader,omitempty"`
	OptimisticFailureTolerance json.Number `json:"optimistic_failure_tolerance,omitempty"`
	// RedundancyZones is ENT only
	RedundancyZones map[string]*RaftAutopilotStateRedundancyZone `json:"redundancy_zones,omitempty"`
	Servers         map[string]*RaftAutopilotStateServer         `json:"servers,omitempty"`
	// UpgradeInfo is ENT only
	UpgradeInfo *RaftAutopilotStateUpgradeInfo `json:"upgrade_info,omitempty"`
	Voters      []string                       `json:"voters,omitempty"`
	NonVoters   []string                       `json:"non_voters,omitempty"`
}

// RaftAutopilotStateRedundancyZone is vault enterprise raft redundancy zone config.
type RaftAutopilotStateRedundancyZone struct {
	Servers          []string    `json:"servers,omitempty"`
	Voters           []string    `json:"voters,omitempty"`
	FailureTolerance json.Number `json:"failure_tolerance,omitempty"`
}

// RaftAutopilotStateServer is the raft autopilot state server.
type RaftAutopilotStateServer struct {
	ID          string          `json:"id,omitempty"`
	Name        string          `json:"name,omitempty"`
	Address     string          `json:"address,omitempty"`
	NodeStatus  string          `json:"node_status,omitempty"`
	LastContact string          `json:"last_contact,omitempty"`
	LastTerm    json.Number     `json:"last_term,omitempty"`
	Healthy     bool            `json:"healthy,omitempty"`
	StableSince string          `json:"stable_since,omitempty"`
	Status      string          `json:"status,omitempty"`
	Meta        json.RawMessage `json:"meta,omitempty"`
}

// RaftAutopilotStateServer is the raft autopilot state upgrade info.
type RaftAutopilotStateUpgradeInfo struct {
	OtherVersionNonVoters  []string                                                `json:"other_version_non_voters,omitempty"`
	OtherVersionVoters     []string                                                `json:"other_version_voters,omitempty"`
	RedundancyZones        map[string]*RaftAutopilotStateUpgradeInfoRedundancyZone `json:"redundancy_zones,omitempty"`
	Status                 string                                                  `json:"status,omitempty"`
	TargetVersion          string                                                  `json:"target_version,omitempty"`
	TargetVersionNonVoters []string                                                `json:"target_version_non_voters,omitempty"`
}

// RaftAutopilotStateServer is the raft autopilot state upgrade info redundancy zone.
type RaftAutopilotStateUpgradeInfoRedundancyZone struct {
	TargetVersionNonVoters []string `json:"target_version_non_voters,omitempty"`
	OtherVersionVoters     []string `json:"other_version_voters,omitempty"`
	OtherVersionNonVoters  []string `json:"other_version_non_voters,omitempty"`
}

// NewRaftConfigurationResponse returns a new instance of RaftConfigurationResponse.
func NewRaftConfigurationResponse() *RaftConfigurationResponse {
	return &RaftConfigurationResponse{
		Data: &RaftConfigurationData{
			Config: &RaftConfigurationDataConfig{
				Servers: []*RaftConfigurationServer{},
			},
		},
	}
}

// NewRaftAutopilotConfigurationResponse returns a new instance of
// RaftAutopilotConfigurationResponse.
func NewRaftAutopilotConfigurationResponse() *RaftAutopilotConfigurationResponse {
	return &RaftAutopilotConfigurationResponse{Data: &RaftAutopilotConfigurationData{}}
}

// NewRaftAutopilotStateResponse returns a new instance of RaftAutopilotStateResponse.
func NewRaftAutopilotStateResponse() *RaftAutopilotStateResponse {
	return &RaftAutopilotStateResponse{
		Data: &RaftAutopilotStateResponseData{
			RedundancyZones: map[string]*RaftAutopilotStateRedundancyZone{},
			Servers:         map[string]*RaftAutopilotStateServer{},
			UpgradeInfo: &RaftAutopilotStateUpgradeInfo{
				OtherVersionNonVoters:  []string{},
				OtherVersionVoters:     []string{},
				RedundancyZones:        map[string]*RaftAutopilotStateUpgradeInfoRedundancyZone{},
				TargetVersionNonVoters: []string{},
			},
			Voters:    []string{},
			NonVoters: []string{},
		},
	}
}

// GetRaftConfiguration returns the vault raft configuration.
func GetRaftConfiguration(ctx context.Context, tr it.Transport, req *CLIRequest) (*RaftConfigurationResponse, error) {
	var err error
	res := NewRaftConfigurationResponse()

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
		err = errors.Join(err, errors.New("you must supply a vault token for the /v1/sys/storage/raft/configuration endpoint"))
	}

	if err == nil {
		stdout, stderr, err1 := tr.Run(ctx, command.New(
			req.BinPath+" read -format=json sys/storage/raft/configuration",
			command.WithEnvVar("VAULT_ADDR", req.VaultAddr),
			command.WithEnvVar("VAULT_TOKEN", req.Token),
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
		return nil, errors.Join(errors.New("get vault raft configuration: vault read sys/storage/raft/configuration"), err)
	}

	return res, nil
}

// GetRaftAutopilotConfiguration returns raft autopilot configuration.
func GetRaftAutopilotConfiguration(ctx context.Context, tr it.Transport, req *CLIRequest) (*RaftAutopilotConfigurationResponse, error) {
	var err error
	res := NewRaftAutopilotConfigurationResponse()

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
		err = errors.Join(err, errors.New("you must supply a vault token for the /v1/sys/storage/raft/autopilot/configuration endpoint"))
	}

	if err == nil {
		stdout, stderr, err1 := tr.Run(ctx, command.New(
			req.BinPath+" read -format=json sys/storage/raft/autopilot/configuration",
			command.WithEnvVar("VAULT_ADDR", req.VaultAddr),
			command.WithEnvVar("VAULT_TOKEN", req.Token),
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
		return nil, errors.Join(errors.New("get vault autopilot configuration: vault read sys/storage/raft/autopilot/configuration"), err)
	}

	return res, nil
}

// GetRaftAutopilotState returns the raft autopilot state.
func GetRaftAutopilotState(ctx context.Context, tr it.Transport, req *CLIRequest) (*RaftAutopilotStateResponse, error) {
	var err error
	res := NewRaftAutopilotStateResponse()

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
		err = errors.Join(err, errors.New("you must supply a vault token for the /v1/sys/storage/raft/autopilot/state endpoint"))
	}

	if err == nil {
		stdout, stderr, err1 := tr.Run(ctx, command.New(
			req.BinPath+" read -format=json sys/storage/raft/autopilot/state",
			command.WithEnvVar("VAULT_ADDR", req.VaultAddr),
			command.WithEnvVar("VAULT_TOKEN", req.Token),
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
		return nil, errors.Join(errors.New("get vault autopilot state: vault read sys/storage/raft/autopilot/state"), err)
	}

	return res, nil
}

// String returns the ha status as a string.
func (s *RaftConfigurationResponse) String() string {
	if s == nil || s.Data == nil {
		return ""
	}

	return s.Data.String()
}

// String returns the seal data as a string.
func (s *RaftConfigurationData) String() string {
	if s == nil || s.Config == nil {
		return ""
	}

	return s.Config.String()
}

// String returns the seal data as a string.
func (s *RaftConfigurationDataConfig) String() string {
	if s == nil {
		return ""
	}

	out := new(strings.Builder)
	_, _ = fmt.Fprintf(out, "Index: %s\n", s.Index)

	if len(s.Servers) < 1 {
		return out.String()
	}

	_, _ = fmt.Fprintln(out, "Servers:")
	for i := range s.Servers {
		_, _ = out.WriteString(istrings.Indent("  ", s.Servers[i].String()))
	}

	return out.String()
}

// String returns the seal data as a string.
func (s *RaftConfigurationServer) String() string {
	if s == nil {
		return ""
	}

	out := new(strings.Builder)
	_, _ = fmt.Fprintln(out, "Server")
	_, _ = fmt.Fprintf(out, "  Address: %s\n", s.Address)
	_, _ = fmt.Fprintf(out, "  Leader: %t\n", s.Leader)
	_, _ = fmt.Fprintf(out, "  Node ID: %s\n", s.NodeID)
	_, _ = fmt.Fprintf(out, "  Protocol Version: %s\n", s.ProtocolVersion)
	_, _ = fmt.Fprintf(out, "  Voter: %t\n", s.Voter)

	return out.String()
}

// String returns the raft autopilot configuration response as a string.
func (s *RaftAutopilotConfigurationResponse) String() string {
	if s == nil || s.Data == nil {
		return ""
	}

	return s.Data.String()
}

// String returns the raft autopilot configuration data as a string.
func (s *RaftAutopilotConfigurationData) String() string {
	if s == nil {
		return ""
	}

	out := new(strings.Builder)
	_, _ = fmt.Fprintf(out, "Cleanup Dead Servers: %t\n", s.CleanupDeadServers)
	_, _ = fmt.Fprintf(out, "Dead Server Last Contact Threshold: %s\n", s.DeadServerLastContactThreshold)
	_, _ = fmt.Fprintf(out, "Last Contact Threshold: %s\n", s.LastContactThreshold)
	_, _ = fmt.Fprintf(out, "Max Trailing Logs: %s\n", s.MaxTrailingLogs)
	_, _ = fmt.Fprintf(out, "Min Quorum: %s\n", s.MinQuorum)
	_, _ = fmt.Fprintf(out, "Server Stabilization Time: %s\n", s.ServerStabilizationTime)
	_, _ = fmt.Fprintf(out, "Disable Upgrade Migration: %t\n", s.DisableUpgradeMigration)

	return out.String()
}

// String returns the RaftAutopilotStateResponse as a string.
func (r *RaftAutopilotStateResponse) String() string {
	if r == nil || r.Data == nil {
		return ""
	}

	return r.Data.String()
}

// String returns the RaftAutopilotStateResponseData as a string.
func (r *RaftAutopilotStateResponseData) String() string {
	if r == nil {
		return ""
	}

	out := new(strings.Builder)
	_, _ = fmt.Fprintf(out, "Healthy: %t\n", r.Healthy)
	_, _ = fmt.Fprintf(out, "Failure Tolerance: %s\n", r.FailureTolerance)
	_, _ = fmt.Fprintf(out, "Leader: %s\n", r.Leader)
	_, _ = fmt.Fprintf(out, "Optimistic Failure Tolerance: %s\n", r.OptimisticFailureTolerance)

	if len(r.RedundancyZones) > 0 {
		_, _ = fmt.Fprintln(out, "Redundancy Zones:")
		for name, val := range r.RedundancyZones {
			_, _ = fmt.Fprintf(out, "  %s\n", name)
			_, _ = out.WriteString(istrings.Indent("  ", val.String()))
		}
	}

	if len(r.Servers) > 0 {
		_, _ = fmt.Fprintln(out, "Servers:")
		for name, val := range r.Servers {
			_, _ = fmt.Fprintf(out, "  %s\n", name)
			_, _ = out.WriteString(istrings.Indent("  ", val.String()))
		}
	}

	if r.UpgradeInfo != nil && r.UpgradeInfo.Status != "" {
		_, _ = fmt.Fprintln(out, "Upgrade Info:")
		_, _ = out.WriteString(istrings.Indent("  ", r.UpgradeInfo.String()))
	}

	for i := range r.Voters {
		if i == 0 {
			_, _ = fmt.Fprintln(out, "Voters")
		}
		_, _ = fmt.Fprintf(out, "  %s\n", r.Voters[i])
	}

	for i := range r.NonVoters {
		if i == 0 {
			_, _ = fmt.Fprintln(out, "Nonvoters")
		}
		_, _ = fmt.Fprintf(out, "  %s\n", r.NonVoters[i])
	}

	return out.String()
}

// String returns the RaftAutopilotStateRedundancyZone as a string.
func (r *RaftAutopilotStateRedundancyZone) String() string {
	if r == nil {
		return ""
	}

	out := new(strings.Builder)

	for i := range r.Servers {
		if i == 0 {
			_, _ = fmt.Fprintln(out, "Servers")
		}
		_, _ = fmt.Fprintf(out, "  %s\n", r.Servers[i])
	}

	for i := range r.Voters {
		if i == 0 {
			_, _ = fmt.Fprintln(out, "Voters")
		}
		_, _ = fmt.Fprintf(out, "  %s\n", r.Voters[i])
	}

	_, _ = fmt.Fprintln(out, r.FailureTolerance)

	return out.String()
}

// String returns the RaftAutopilotStateServer as a string.
func (r *RaftAutopilotStateServer) String() string {
	if r == nil {
		return ""
	}

	out := new(strings.Builder)
	_, _ = fmt.Fprintf(out, "ID: %s\n", r.ID)
	_, _ = fmt.Fprintf(out, "Name: %s\n", r.Name)
	_, _ = fmt.Fprintf(out, "Address: %s\n", r.Address)
	_, _ = fmt.Fprintf(out, "Node Status: %s\n", r.NodeStatus)
	_, _ = fmt.Fprintf(out, "Last Contact: %s\n", r.LastContact)
	_, _ = fmt.Fprintf(out, "Last Term: %s\n", r.LastTerm)
	_, _ = fmt.Fprintf(out, "Healthy: %t\n", r.Healthy)
	_, _ = fmt.Fprintf(out, "Stable Since: %s\n", r.StableSince)
	_, _ = fmt.Fprintf(out, "Status: %s\n", r.Status)
	_, _ = fmt.Fprintf(out, "Meta: %s\n", r.Meta)

	return out.String()
}

// String returns the RaftAutopilotStateUpgradeInfo as a string.
func (r *RaftAutopilotStateUpgradeInfo) String() string {
	if r == nil {
		return ""
	}
	out := new(strings.Builder)

	for i := range r.OtherVersionNonVoters {
		if i == 0 {
			_, _ = fmt.Fprintln(out, "Other Version Nonvoters")
		}
		_, _ = fmt.Fprintf(out, "  %s\n", r.OtherVersionNonVoters[i])
	}

	for i := range r.OtherVersionVoters {
		if i == 0 {
			_, _ = fmt.Fprintln(out, "Other Version Voters")
		}
		_, _ = fmt.Fprintf(out, "  %s\n", r.OtherVersionVoters[i])
	}

	if len(r.RedundancyZones) > 0 {
		_, _ = fmt.Fprintln(out, "Redundancy Zones:")
	}
	for name, val := range r.RedundancyZones {
		_, _ = fmt.Fprintf(out, "  %s\n", name)
		_, _ = out.WriteString(istrings.Indent("  ", val.String()))
	}

	_, _ = fmt.Fprintf(out, "Status: %s\n", r.Status)
	_, _ = fmt.Fprintf(out, "Target Version: %s\n", r.TargetVersion)

	for i := range r.TargetVersionNonVoters {
		if i == 0 {
			_, _ = fmt.Fprintln(out, "Target Version Nonvoters")
		}
		_, _ = fmt.Fprintf(out, "  %s\n", r.TargetVersionNonVoters[i])
	}

	return out.String()
}

// String returns the RaftAutopilotStateUpgradeInfoRedundancyZone as a string.
func (r *RaftAutopilotStateUpgradeInfoRedundancyZone) String() string {
	if r == nil {
		return ""
	}
	out := new(strings.Builder)

	for i := range r.TargetVersionNonVoters {
		if i == 0 {
			_, _ = fmt.Fprintln(out, "Target Version Nonvoters")
		}
		_, _ = fmt.Fprintf(out, "  %s\n", r.TargetVersionNonVoters[i])
	}

	for i := range r.OtherVersionVoters {
		if i == 0 {
			_, _ = fmt.Fprintln(out, "Other Version Voters")
		}
		_, _ = fmt.Fprintf(out, "  %s\n", r.OtherVersionVoters[i])
	}

	for i := range r.OtherVersionNonVoters {
		if i == 0 {
			_, _ = fmt.Fprintln(out, "Other Version Nonvoters")
		}
		_, _ = fmt.Fprintf(out, "  %s\n", r.OtherVersionNonVoters[i])
	}

	return out.String()
}
