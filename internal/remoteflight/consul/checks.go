// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"errors"
	"fmt"

	"github.com/hashicorp/enos-provider/internal/remoteflight/systemd"
)

// CheckStateHasSystemdEnabledAndRunningProperties checks that the consul systemd service
// has all of the properties and values we expect for a service to be running.
func CheckStateHasSystemdEnabledAndRunningProperties() CheckStater {
	return func(s *State) error {
		props, err := s.UnitProperties.FindProperties(systemd.EnabledAndRunningProperties)
		if err != nil {
			return fmt.Errorf("expected consul systemd unit to be enabled and running, got: %s", err)
		}

		if !props.HasProperties(systemd.EnabledAndRunningProperties) {
			return fmt.Errorf("expected consul system unit to be enabled and running, got: %s", props.String())
		}

		return nil
	}
}

// CheckStateNodeIsHealthy checks whether or not the consul node is healthy.
func CheckStateNodeIsHealthy() CheckStater {
	return func(s *State) error {
		if s.HealthNodeResponse == nil || s.HealthNodeResponse.Nodes == nil {
			return errors.New("node health was not found in state")
		}

		if n := len(s.HealthNodeResponse.Nodes); n != 1 {
			return fmt.Errorf("expected one node health node, got: %d", n)
		}

		if n := s.HealthNodeResponse.Nodes[0]; n.Status != NodeHealthStatusHealthy {
			return fmt.Errorf("expected node health status to be: %s, got: %s", NodeHealthStatusHealthy, n.String())
		}

		return nil
	}
}

// CheckStateClusterHasLeader checks whether or not the consul cluster has a leader.
func CheckStateClusterHasLeader() CheckStater {
	return func(s *State) error {
		if s.RaftConfigurationResponse == nil || s.RaftConfigurationResponse.Servers == nil {
			return errors.New("no raft servers were found in state")
		}

		for i := range s.RaftConfigurationResponse.Servers {
			i := i
			if s.RaftConfigurationResponse.Servers[i].Leader {
				return nil
			}
		}

		return fmt.Errorf("no raft servers are leader, got response: %v",
			s.RaftConfigurationResponse.String(),
		)
	}
}

// CheckStateClusterHasMinNHealthyNodes checks whether or not the cluster has a minimum
// of N healthy nodes.
func CheckStateClusterHasMinNHealthyNodes(min uint) CheckStater {
	return func(s *State) error {
		if s.HealthStatePassingResponse == nil || s.HealthStatePassingResponse.Nodes == nil {
			return errors.New("node health was not found in state")
		}

		healthy := uint(0)
		for i := range s.HealthStatePassingResponse.Nodes {
			i := i
			if s.HealthStatePassingResponse.Nodes[i].Status == NodeHealthStatusHealthy {
				healthy++
			}
		}

		if healthy >= min {
			return nil
		}

		return fmt.Errorf("expected minimum of %d healthy nodes, got %d, response: %s",
			min, healthy, s.HealthStatePassingResponse.String(),
		)
	}
}

// CheckStateClusterHasMinNVoters checks whether or not the cluster has a minimum of
// N raft voters.
func CheckStateClusterHasMinNVoters(min uint) CheckStater {
	return func(s *State) error {
		if s.RaftConfigurationResponse == nil || s.RaftConfigurationResponse.Servers == nil {
			return errors.New("no raft servers were found in state")
		}

		voters := uint(0)
		for i := range s.RaftConfigurationResponse.Servers {
			i := i
			if s.RaftConfigurationResponse.Servers[i].Voter {
				voters++
			}
		}

		if voters >= min {
			return nil
		}

		return fmt.Errorf("expected minimum of %d raft voters, got %d, response: %s",
			min, voters, s.RaftConfigurationResponse.String(),
		)
	}
}
