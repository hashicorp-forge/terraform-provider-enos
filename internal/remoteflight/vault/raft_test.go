// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRaftConfigurationDeserialize(t *testing.T) {
	t.Parallel()
	expected := NewRaftConfigurationResponse()
	expected.Data.Config.Index = "0"
	expected.Data.Config.Servers = []*RaftConfigurationServer{
		{
			Address:         "10.13.10.239:8201",
			Leader:          true,
			NodeID:          "node_0",
			ProtocolVersion: "3",
			Voter:           true,
		},
		{
			Address:         "10.13.10.150:8201",
			Leader:          false,
			NodeID:          "node_1",
			ProtocolVersion: "3",
			Voter:           true,
		},
		{
			Address:         "10.13.10.114:8201",
			Leader:          false,
			NodeID:          "node_2",
			ProtocolVersion: "3",
			Voter:           true,
		},
	}

	got := NewRaftConfigurationResponse()
	body := testReadSupport(t, "storage-raft-configuration.json")
	require.NoError(t, json.Unmarshal(body, got))

	require.Equal(t, expected, got)
}

func TestRaftAutopilotConfigurationDeserialize(t *testing.T) {
	t.Parallel()
	expected := NewRaftAutopilotConfigurationResponse()
	expected.Data.DeadServerLastContactThreshold = "24h0m0s"
	expected.Data.LastContactThreshold = "10s"
	expected.Data.MaxTrailingLogs = "1000"
	expected.Data.MinQuorum = "0"
	expected.Data.ServerStabilizationTime = "10s"

	got := NewRaftAutopilotConfigurationResponse()
	body := testReadSupport(t, "storage-raft-autopilot-configuration.json")
	require.NoError(t, json.Unmarshal(body, got))

	require.Equal(t, expected, got)
}

func TestRaftAutopilotStateDeserialize(t *testing.T) {
	t.Parallel()
	expected := NewRaftAutopilotStateResponse()
	expected.Data.Healthy = true
	expected.Data.FailureTolerance = "1"
	expected.Data.Leader = "node_0"
	expected.Data.OptimisticFailureTolerance = "1"
	expected.Data.Servers = map[string]*RaftAutopilotStateServer{
		"node_0": {
			ID:          "node_0",
			Name:        "node_0",
			Address:     "10.13.10.239:8201",
			NodeStatus:  "alive",
			LastContact: "0s",
			LastTerm:    "3",
			Healthy:     true,
			StableSince: "2023-05-10T21:34:05.142269016Z",
			Status:      "leader",
		},
		"node_1": {
			ID:          "node_1",
			Name:        "node_1",
			Address:     "10.13.10.150:8201",
			NodeStatus:  "alive",
			LastContact: "1.345194439s",
			LastTerm:    "3",
			Healthy:     true,
			StableSince: "2023-05-10T21:34:07.143318399Z",
			Status:      "voter",
		},
		"node_2": {
			ID:          "node_2",
			Name:        "node_2",
			Address:     "10.13.10.114:8201",
			NodeStatus:  "alive",
			LastContact: "1.23103906s",
			LastTerm:    "3",
			Healthy:     true,
			StableSince: "2023-05-10T21:34:07.143318399Z",
			Status:      "voter",
		},
	}
	expected.Data.UpgradeInfo.Status = "idle"
	expected.Data.UpgradeInfo.TargetVersion = "1.14.0"
	expected.Data.Voters = []string{
		"node_0",
		"node_1",
		"node_2",
	}

	got := NewRaftAutopilotStateResponse()
	body := testReadSupport(t, "storage-raft-autopilot-state.json")
	require.NoError(t, json.Unmarshal(body, got))

	require.Equal(t, expected, got)
}
