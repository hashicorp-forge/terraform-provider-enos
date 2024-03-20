// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"

	"github.com/hashicorp/enos-provider/internal/remoteflight/systemd"
)

// CheckStateHasStatusCode checks that the vault status code matches the given code.
func CheckStateHasStatusCode(status StatusCode) CheckStater {
	return func(s *State) error {
		if s.Status.StatusCode != status {
			return fmt.Errorf("checking vault status code, expected status %v, got %v", status, s.Status.StatusCode)
		}

		return nil
	}
}

// CheckStateHasSystemdEnabledAndRunningProperties checks that the vault systemd service
// has all of the properties and values we expect for a service to be running.
func CheckStateHasSystemdEnabledAndRunningProperties() CheckStater {
	return func(s *State) error {
		props, err := s.UnitProperties.FindProperties(systemd.EnabledAndRunningProperties)
		if err != nil {
			return fmt.Errorf("checking vault systemd state, expected vault systemd unit to be enabled and running, got %s", err)
		}

		if !props.HasProperties(systemd.EnabledAndRunningProperties) {
			return fmt.Errorf("checking vault systemd state, expected vault system unit to be enabled and running, got %s", props.String())
		}

		return nil
	}
}

// CheckStateAllPodsHavePhase takes a phase and asserts that all of the pods match the phase.
func CheckStateAllPodsHavePhase(phase v1.PodPhase) CheckStater {
	return func(s *State) error {
		if s == nil ||
			s.PodList == nil ||
			s.PodList.Pods == nil ||
			s.PodList.Pods.Items == nil ||
			len(s.PodList.Pods.Items) < 1 {
			return errors.New("checking kubernetes for pod phases: no pods found in state")
		}

		var err error
		for i := range s.PodList.Pods.Items {
			if s.PodList.Pods.Items[i].Status.Phase != phase {
				err = errors.Join(err, fmt.Errorf("expected pod %s to have phase %s, got %s, message: %s, reason: %s",
					s.PodList.Pods.Items[i].ObjectMeta.Name,
					phase,
					s.PodList.Pods.Items[i].Status.Phase,
					s.PodList.Pods.Items[i].Status.Message,
					s.PodList.Pods.Items[i].Status.Reason,
				))
			}
		}

		if err != nil {
			return errors.Join(fmt.Errorf("checking all kubernetes pods have phase %s", phase), err)
		}

		return nil
	}
}

// CheckStatePodHasPhase takes a pod name and a phase and asserts that the pod has the expected
// phase.
func CheckStatePodHasPhase(name string, phase v1.PodPhase) CheckStater {
	return func(s *State) error {
		if s == nil ||
			s.PodList == nil ||
			s.PodList.Pods == nil ||
			s.PodList.Pods.Items == nil ||
			len(s.PodList.Pods.Items) < 1 {
			return fmt.Errorf("checking kubernetes for pod %s phase: no pods found in state", name)
		}

		for i := range s.PodList.Pods.Items {
			if s.PodList.Pods.Items[i].Name != name {
				continue
			}

			if s.PodList.Pods.Items[i].Status.Phase != phase {
				return fmt.Errorf("expected pod %s to have phase %s, got %s, message: %s, reason: %s",
					name,
					phase,
					s.PodList.Pods.Items[i].Status.Phase,
					s.PodList.Pods.Items[i].Status.Message,
					s.PodList.Pods.Items[i].Status.Reason,
				)
			}

			return nil
		}

		return fmt.Errorf("checking kubernetes for pod %s phase: no pods named %[1]s found in state", name)
	}
}

// CheckStateAllContainersAreReady checks that all containers found in the state are Ready.
func CheckStateAllContainersAreReady() CheckStater {
	return func(s *State) error {
		if s == nil ||
			s.PodList == nil ||
			s.PodList.Pods == nil ||
			s.PodList.Pods.Items == nil ||
			len(s.PodList.Pods.Items) < 1 {
			return errors.New("checking all containers in kubernetes states are Ready")
		}

		var err error
		for i := range s.PodList.Pods.Items {
			for ic := range s.PodList.Pods.Items[i].Status.ContainerStatuses {
				if !s.PodList.Pods.Items[i].Status.ContainerStatuses[ic].Ready {
					err = errors.Join(fmt.Errorf("container %s is not ready",
						s.PodList.Pods.Items[i].Status.ContainerStatuses[ic].Name,
					))
				}
			}
		}

		if err != nil {
			return errors.Join(
				errors.New("checking all containers in kubernetes state are ready"),
				err,
			)
		}

		return nil
	}
}

// CheckStateAllPodContainersAreReady takes a pod name and asserts that all of its containers are ready.
func CheckStateAllPodContainersAreReady(podName string) CheckStater {
	return func(s *State) error {
		if s == nil ||
			s.PodList == nil ||
			s.PodList.Pods == nil ||
			s.PodList.Pods.Items == nil ||
			len(s.PodList.Pods.Items) < 1 {
			return fmt.Errorf("checking all containers in kubernetes pod %s are Ready", podName)
		}

		for i := range s.PodList.Pods.Items {
			if s.PodList.Pods.Items[i].Name != podName {
				continue
			}

			var err error
			for ic := range s.PodList.Pods.Items[i].Status.ContainerStatuses {
				if !s.PodList.Pods.Items[i].Status.ContainerStatuses[ic].Ready {
					err = errors.Join(fmt.Errorf("container %s is not ready",
						s.PodList.Pods.Items[i].Status.ContainerStatuses[ic].Name,
					))
				}
			}
			if err != nil {
				return errors.Join(
					fmt.Errorf("checking all containers in kubernetes pod %s are ready", podName),
					err,
				)
			}

			return nil
		}

		return fmt.Errorf(
			"checking all containers in kubernetes pod %s are in Ready state: pod not found in state",
			podName,
		)
	}
}

// CheckStatePodContainerIsReady takes a pod name, a container name and asserts that the container
// is ready.
func CheckStatePodContainerIsReady(podName string, containerName string) CheckStater {
	return func(s *State) error {
		if s == nil ||
			s.PodList == nil ||
			s.PodList.Pods == nil ||
			s.PodList.Pods.Items == nil ||
			len(s.PodList.Pods.Items) < 1 {
			return fmt.Errorf(
				"checking kubernetes for pod %s container %s for Ready state: no pods found in state",
				podName, containerName,
			)
		}

		for i := range s.PodList.Pods.Items {
			if s.PodList.Pods.Items[i].Name != podName {
				continue
			}

			if s.PodList.Pods.Items[i].Status.ContainerStatuses == nil {
				continue
			}

			for ic := range s.PodList.Pods.Items[i].Status.ContainerStatuses {
				if s.PodList.Pods.Items[i].Status.ContainerStatuses[ic].Name != containerName {
					continue
				}

				if !s.PodList.Pods.Items[i].Status.ContainerStatuses[ic].Ready {
					return fmt.Errorf(
						"checking pod %s container %s for Ready state: container is not Ready",
						podName, containerName,
					)
				}

				return nil
			}
		}

		return fmt.Errorf(
			"checking kubernetes for pod %s container %s for Ready state: no pod/container found in state",
			podName, containerName,
		)
	}
}

// CheckStateIsInitialized checks whether or not vault is initialized.
func CheckStateIsInitialized() CheckStater {
	return func(s *State) error {
		initialized, err := s.IsInitialized()
		if err != nil {
			return fmt.Errorf("checking vault init state: %s", err)
		}

		if initialized {
			return nil
		}

		return errors.New("checking vault init state, expected initialized, vault is not initialized")
	}
}

// CheckStateIsUnsealed checks whether or not the Vault node is unsealed.
func CheckStateIsUnsealed() CheckStater {
	return func(s *State) error {
		sealed, err := s.IsSealed()
		if err != nil {
			return fmt.Errorf("checking vault seal state: %s", err)
		}

		if sealed {
			return errors.New("checking vault seal state, expected unsealed, vault is sealed")
		}

		return nil
	}
}

// CheckStateIsSealed checks whether or not the Vault node is sealed.
func CheckStateIsSealed() CheckStater {
	return func(s *State) error {
		sealed, err := s.IsSealed()
		if err != nil {
			return fmt.Errorf("checking vault seal state: %s", err)
		}

		if !sealed {
			return errors.New("checking vault seal state, expected sealed, vault is unsealed")
		}

		return nil
	}
}

// CheckStateSealStateIsKnown checks whether or not the Vault node has a valid
// seal state.
func CheckStateSealStateIsKnown() CheckStater {
	return func(s *State) error {
		_, err := s.IsSealed()
		if err != nil {
			return fmt.Errorf("checking vault seal state: %s", err)
		}

		return nil
	}
}

// CheckStateHasSealType checks whether or not the node has the given seal type.
func CheckStateHasSealType(stype SealType) CheckStater {
	return func(s *State) error {
		if s.SealStatus == nil || s.SealStatus.Data == nil {
			return fmt.Errorf("checking vault seal type, expected %s, no seal status data was found in state", stype)
		}

		if s.SealStatus.Data.Type == stype {
			return nil
		}

		return fmt.Errorf(
			"checking vault seal type, expected type %s, found %s",
			stype, s.SealStatus.Data.Type,
		)
	}
}

// CheckStateHasHealthStatusOf takes one-or-more health statuses and checks
// whether or not the node has one of the health status.
func CheckStateHasHealthStatusOf(statuses ...HealthStatus) CheckStater {
	return func(s *State) error {
		if len(statuses) < 1 {
			return errors.New("checking vault health status: no desired statuses were given")
		}

		if s.Health == nil {
			return errors.New("checking vault health status: state has no /v1/sys/health data")
		}

		if s.Health.StatusIsOneOf(statuses...) {
			return nil
		}

		return fmt.Errorf("checking vault health status: expected one of /v1/sys/health statuses: %v, got /v1/sys/health status: %s",
			statuses,
			s.Health.Status(),
		)
	}
}

// CheckStateHasEnableUIInConfig checks whether or not the vault cluster has been
// configured to enable the UI.
func CheckStateHasEnableUIInConfig() CheckStater {
	return func(s *State) error {
		if s.ConfigSanitized == nil || s.ConfigSanitized.Data == nil {
			return errors.New("checking if vault UI is enabled: no configuration data was found in state")
		}

		if s.ConfigSanitized.Data.EnableUI {
			return nil
		}

		return errors.New("checking vault UI is enabled: vault UI is disabled")
	}
}

// CheckStateHasMatchingListenerInConfig checks whether or not the vault cluster has been
// configured with a matching listener.
func CheckStateHasMatchingListenerInConfig(listener *ConfigListener) CheckStater {
	return func(s *State) error {
		if s.ConfigSanitized == nil || s.ConfigSanitized.Data == nil {
			return errors.New("checking if vault config has matching listener: no configuration data was found in state")
		}

		for i := range s.ConfigSanitized.Data.Listeners {
			i := i

			if s.ConfigSanitized.Data.Listeners[i].Type != listener.Type {
				continue
			}

			if s.ConfigSanitized.Data.Listeners[i].Config.Address != listener.Config.Address {
				continue
			}

			if s.ConfigSanitized.Data.Listeners[i].Config.TLSDisable != listener.Config.TLSDisable {
				continue
			}

			// We found a matching listener
			return nil
		}

		return fmt.Errorf(
			"checking if vault config has matching listener: no matching listener config was found in state, expected: %#v, got: %#v",
			listener, s.ConfigSanitized.Data.Listeners,
		)
	}
}

// CheckStateHasHAActiveNode checks whether or not the vault cluster has an active
// HA node.
func CheckStateHasHAActiveNode() CheckStater {
	return func(s *State) error {
		if s.HAStatus == nil || s.HAStatus.Data == nil || s.HAStatus.Data.Nodes == nil {
			return errors.New("checking vault HA status for active node: no active nodes were found in state")
		}

		for i := range s.HAStatus.Data.Nodes {
			i := i
			if s.HAStatus.Data.Nodes[i].ActiveNode {
				return nil
			}
		}

		return errors.New("checking vault HA status for active node: no active nodes were found in state")
	}
}

// CheckStateHasMinNHANodes checks whether or not the cluster has a minimum
// of N nodes.
func CheckStateHasMinNHANodes(min uint) CheckStater {
	return func(s *State) error {
		if (s.HAStatus == nil || s.HAStatus.Data == nil) && min > 0 {
			return fmt.Errorf("checking if vault has minimum of %d HA nodes: no HA status was found in state", min)
		}

		if l := len(s.HAStatus.Data.Nodes); uint(l) < min {
			return fmt.Errorf("checking if vault has minimum of %d HA nodes: expected at least %[1]d HA nodes, got %d", min, l)
		}

		return nil
	}
}

// CheckStateHasMinNRaftServers checks whether or not the cluster has a minimum
// of N raft servers.
func CheckStateHasMinNRaftServers(min uint) CheckStater {
	return func(s *State) error {
		if (s.RaftConfig == nil || s.RaftConfig.Data == nil || s.RaftConfig.Data.Config == nil) && min > 0 {
			return fmt.Errorf("checking if vault has minimum of %d raft servers: no raft config data was found in state", min)
		}

		if l := len(s.RaftConfig.Data.Config.Servers); uint(l) < min {
			return fmt.Errorf("checking if vault has minimum of %d raft servers: expected %[1]d raft servers, got %d", min, l)
		}

		return nil
	}
}

// CheckStateHasMinNRaftVoters checks whether or not the cluster has a minimum
// of N raft voters.
func CheckStateHasMinNRaftVoters(min uint) CheckStater {
	return func(s *State) error {
		if (s.RaftConfig == nil || s.RaftConfig.Data == nil || s.RaftConfig.Data.Config == nil) && min > 0 {
			return fmt.Errorf("checking if vault has minimum of %d raft voters: no raft config data was found in state", min)
		}

		voters := uint(0)
		for i := range s.RaftConfig.Data.Config.Servers {
			i := i
			if s.RaftConfig.Data.Config.Servers[i].Voter {
				voters++
			}
		}

		if voters >= min {
			return nil
		}

		return fmt.Errorf("checking if vault has minimum of %d raft voters: expected %[1]d raft servers, got %d", min, voters)
	}
}

// CheckStateHasRaftLeader checks whether or not the cluster has a raft leader.
func CheckStateHasRaftLeader() CheckStater {
	return func(s *State) error {
		if s.RaftConfig == nil || s.RaftConfig.Data == nil || s.RaftConfig.Data.Config == nil {
			return errors.New("checking if vault has a raft leader: no raft config data was found in state")
		}

		for i := range s.RaftConfig.Data.Config.Servers {
			i := i
			if s.RaftConfig.Data.Config.Servers[i].Leader {
				return nil
			}
		}

		return errors.New("checking if vault has a raft leader: unable to find raft leader")
	}
}

// CheckStateHasMinNAutopilotServers checks whether or not the cluster has a minimum
// of N autopilot servers.
func CheckStateHasMinNAutopilotServers(min uint) CheckStater {
	return func(s *State) error {
		if (s.AutopilotState == nil || s.AutopilotState.Data == nil) && min > 0 {
			return fmt.Errorf("checking if vault has minimum of %d autopilot servers: no autopilot data was found in state", min)
		}

		if l := len(s.AutopilotState.Data.Servers); uint(l) < min {
			return fmt.Errorf("checking if vault has minimum of %d autopilot servers: expected %[1]d servers, got %d", min, l)
		}

		return nil
	}
}

// CheckStateHasMinNAutopilotVoters checks whether or not the cluster has a minimum
// of N autopilot voters.
func CheckStateHasMinNAutopilotVoters(min uint) CheckStater {
	return func(s *State) error {
		if (s.AutopilotState == nil || s.AutopilotState.Data == nil) && min > 0 {
			return fmt.Errorf("checking if vault has minimum of %d autopilot voters: no autopilot data was found in state", min)
		}

		if l := len(s.AutopilotState.Data.Voters); uint(l) < min {
			return fmt.Errorf("checking if vault has minimum of %d autopilot voters: expected %[1]d autopilot voters, got %d", min, l)
		}

		return nil
	}
}

// CheckStateHasMinNAutopilotHealthyNodes checks whether or not the cluster has a minimum
// of N autopilot healthy nodes.
func CheckStateHasMinNAutopilotHealthyNodes(min uint) CheckStater {
	return func(s *State) error {
		if (s.AutopilotState == nil || s.AutopilotState.Data == nil) && min > 0 {
			return fmt.Errorf("checking if vault has minimum of %d autopilot healthy nodes: no autopilot data was found in state", min)
		}

		healthy := uint(0)
		for i := range s.AutopilotState.Data.Servers {
			i := i
			if s.AutopilotState.Data.Servers[i].Healthy {
				healthy++
			}
		}

		if healthy < min {
			return fmt.Errorf(
				"checking if vault has minimum of %d autopilot healthy nodes: expected %[1]d autopilot voters, got %d",
				min, healthy,
			)
		}

		return nil
	}
}

// CheckStateAutopilotIsHealthy checks whether or not the autopilot is in a
// healthy state.
func CheckStateAutopilotIsHealthy() CheckStater {
	return func(s *State) error {
		if s.AutopilotState == nil || s.AutopilotState.Data == nil {
			return errors.New("checking if vault autopilot is healthy: no autopilot data was found in state")
		}

		if s.AutopilotState.Data.Healthy {
			return nil
		}

		return errors.New("checking if vault autopilot is healthy: autopilot is not healthy")
	}
}

// CheckStateAutopilotHasLeader checks whether or not the cluster has a raft leader.
func CheckStateAutopilotHasLeader() CheckStater {
	return func(s *State) error {
		if s.AutopilotState == nil || s.AutopilotState.Data == nil {
			return errors.New("checking if vault has an autopilot leader: no autopilot data was found in state")
		}

		for i := range s.AutopilotState.Data.Servers {
			i := i
			if s.AutopilotState.Data.Servers[i].Status == "leader" {
				if s.AutopilotState.Data.Servers[i].Name != s.AutopilotState.Data.Leader {
					return fmt.Errorf("checking if vault has an autopilot leader: mismatch on leader, servers data has %s, data has %s",
						s.AutopilotState.Data.Servers[i].Name, s.AutopilotState.Data.Leader,
					)
				}

				return nil
			}
		}

		return errors.New("checking if vault has an autopilot leader: unable to find autopilot leader")
	}
}

// CheckStateHasStorageType checks whether or not the node has the given storage type.
func CheckStateHasStorageType(stype string) CheckStater {
	return func(s *State) error {
		if s.SealStatus == nil || s.SealStatus.Data == nil {
			return fmt.Errorf("checking vault storage type, expected %s, no seal status storage data was found in state", stype)
		}

		if s.SealStatus.Data.StorageType == stype {
			return nil
		}

		return fmt.Errorf(
			"checking vault storage type, expected of %s, found %s",
			stype, s.SealStatus.Data.StorageType,
		)
	}
}
