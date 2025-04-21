// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight/systemd"
)

func TestStateHasSystemdEnabledAndRunningProperties(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		props      systemd.UnitProperties
		shouldFail bool
	}{
		"enabled": {
			systemd.EnabledAndRunningProperties,
			false,
		},
		"not-loaded": {
			systemd.UnitProperties{
				"LoadState":   "not-found",
				"ActiveState": "inactive",
				"SubState":    "dead",
			},
			true,
		},
		"activating": {
			systemd.UnitProperties{
				"LoadState":   "loaded",
				"ActiveState": "activating",
				"SubState":    "dead",
			},
			true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := NewState()
			state.UnitProperties = test.props
			if test.shouldFail {
				require.Error(t, CheckStateHasSystemdEnabledAndRunningProperties()(state))
			} else {
				require.NoError(t, CheckStateHasSystemdEnabledAndRunningProperties()(state))
			}
		})
	}
}

func TestStateNodeIsHealthy(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-health-in-state": {
			func(*testing.T) *State { return NewState() },
			true,
		},
		"too-many-nodes": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "health-node.json")
				state := NewState()
				state.HealthNodeResponse = &HealthNodeResponse{Nodes: make([]*NodeHealth, 0)}
				require.NoError(t, json.Unmarshal(content, &state.HealthNodeResponse.Nodes))
				cpy := *state.HealthNodeResponse.Nodes[0]
				state.HealthNodeResponse.Nodes = append(state.HealthNodeResponse.Nodes, &cpy)

				return state
			},
			true,
		},
		"unhealthy": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "health-node.json")
				state := NewState()
				state.HealthNodeResponse = &HealthNodeResponse{Nodes: make([]*NodeHealth, 0)}
				require.NoError(t, json.Unmarshal(content, &state.HealthNodeResponse.Nodes))
				state.HealthNodeResponse.Nodes[0].Status = "critical"

				return state
			},
			true,
		},
		"healthy": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "health-node.json")
				state := NewState()
				state.HealthNodeResponse = &HealthNodeResponse{Nodes: make([]*NodeHealth, 0)}
				require.NoError(t, json.Unmarshal(content, &state.HealthNodeResponse.Nodes))

				return state
			},
			false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateNodeIsHealthy()(state))
			} else {
				require.NoError(t, CheckStateNodeIsHealthy()(state))
			}
		})
	}
}

func TestStateClusterHasLeader(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-servers-in-state": {
			func(*testing.T) *State { return NewState() },
			true,
		},
		"no-leader": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "raft-configuration.json")
				state := NewState()
				state.RaftConfigurationResponse = &RaftConfigurationResponse{Servers: make([]*RaftServer, 0)}
				require.NoError(t, json.Unmarshal(content, &state.RaftConfigurationResponse))
				state.RaftConfigurationResponse.Servers[0].Leader = false

				return state
			},
			true,
		},
		"healthy": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "raft-configuration.json")
				state := NewState()
				state.RaftConfigurationResponse = &RaftConfigurationResponse{Servers: make([]*RaftServer, 0)}
				require.NoError(t, json.Unmarshal(content, &state.RaftConfigurationResponse))

				return state
			},
			false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateClusterHasLeader()(state))
			} else {
				require.NoError(t, CheckStateClusterHasLeader()(state))
			}
		})
	}
}

func TestStateClusterHasMinNHealthyNodes(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-health-in-state": {
			func(*testing.T) *State { return NewState() },
			true,
		},
		"more-than-required-nodes": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "health-passing.json")
				state := NewState()
				state.HealthStatePassingResponse = &HealthStatePassingResponse{Nodes: make([]*NodeHealth, 0)}
				require.NoError(t, json.Unmarshal(content, &state.HealthStatePassingResponse.Nodes))
				cpy := *state.HealthStatePassingResponse.Nodes[0]
				state.HealthStatePassingResponse.Nodes = append(state.HealthStatePassingResponse.Nodes, &cpy)

				return state
			},
			false,
		},
		"not-enough-nodes": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "health-passing.json")
				state := NewState()
				state.HealthStatePassingResponse = &HealthStatePassingResponse{Nodes: make([]*NodeHealth, 0)}
				require.NoError(t, json.Unmarshal(content, &state.HealthStatePassingResponse.Nodes))
				state.HealthStatePassingResponse.Nodes = state.HealthStatePassingResponse.Nodes[1:]

				return state
			},
			true,
		},
		"exactly-n-nodes": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "health-passing.json")
				state := NewState()
				state.HealthStatePassingResponse = &HealthStatePassingResponse{Nodes: make([]*NodeHealth, 0)}
				require.NoError(t, json.Unmarshal(content, &state.HealthStatePassingResponse.Nodes))

				return state
			},
			false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateClusterHasMinNHealthyNodes(3)(state))
			} else {
				require.NoError(t, CheckStateClusterHasMinNHealthyNodes(3)(state))
			}
		})
	}
}

func TestStateClusterHasMinNVoters(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		state      func(*testing.T) *State
		shouldFail bool
	}{
		"no-servers-in-state": {
			func(*testing.T) *State { return NewState() },
			true,
		},
		"more-than-required-voters": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "raft-configuration.json")
				state := NewState()
				state.RaftConfigurationResponse = &RaftConfigurationResponse{Servers: make([]*RaftServer, 0)}
				require.NoError(t, json.Unmarshal(content, &state.RaftConfigurationResponse))
				cpy := *state.Servers[0]
				state.Servers = append(state.Servers, &cpy)

				return state
			},
			false,
		},
		"not-enough-voters": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "raft-configuration.json")
				state := NewState()
				state.RaftConfigurationResponse = &RaftConfigurationResponse{Servers: make([]*RaftServer, 0)}
				require.NoError(t, json.Unmarshal(content, &state.RaftConfigurationResponse))
				state.RaftConfigurationResponse.Servers[0].Voter = false

				return state
			},
			true,
		},
		"exactly-n-voters": {
			func(t *testing.T) *State {
				t.Helper()
				content := testReadSupport(t, "raft-configuration.json")
				state := NewState()
				state.RaftConfigurationResponse = &RaftConfigurationResponse{Servers: make([]*RaftServer, 0)}
				require.NoError(t, json.Unmarshal(content, &state.RaftConfigurationResponse))

				return state
			},
			false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			state := test.state(t)
			if test.shouldFail {
				require.Error(t, CheckStateClusterHasMinNVoters(3)(state))
			} else {
				require.NoError(t, CheckStateClusterHasMinNVoters(3)(state))
			}
		})
	}
}

func testReadSupport(t *testing.T, name string) []byte {
	t.Helper()

	p, err := filepath.Abs(filepath.Join("./support", name))
	require.NoError(t, err)
	f, err := os.Open(p)
	require.NoError(t, err)
	content, err := io.ReadAll(f)
	require.NoError(t, err)

	return content
}
