// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/kubernetes"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/log"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight/systemd"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/retry"
	istrings "github.com/hashicorp-forge/terraform-provider-enos/internal/strings"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/k8s"
)

// State represents the state of a node in a vault cluster.
type State struct {
	AutopilotConfig   *RaftAutopilotConfigurationResponse // /v1/sys/storage/raft/autopilot/configuration
	AutopilotState    *RaftAutopilotStateResponse         // /v1/sys/storage/raft/autopilot/state
	ConfigSanitized   *ConfigStateSanitizedResponse       // /v1/sys/config/state/sanitized
	ReplicationStatus *ReplicationResponse                // /v1/sys/replication/status
	Health            *HealthResponse                     // /v1/sys/health
	HAStatus          *HAStatusResponse                   // /v1/sys/ha-status
	HostInfo          *HostInfoResponse                   // /v1/sys/host-info
	PerfReplication   *ReplicationResponse                // /v1/sys/replication/performance
	PodList           *kubernetes.ListPodsResponse        // kubernetes pod info for vault pod
	RaftConfig        *RaftConfigurationResponse          // /v1/sys/storage/raft/configuration
	SealStatus        *SealStatusResponse                 // /v1/sys/seal-status
	Status            *StatusResponse                     // "vault status"
	UnitProperties    systemd.UnitProperties              // systemd unit properties for vault.service
}

// CheckStater is a validate function that takes a state and validates that it
// has expected values.
type CheckStater func(s *State) error

// StateRequest is a vault state request.
type StateRequest struct {
	// Basic vault binary information
	*CLIRequest
	// Where to install enos-flight-control
	FlightControlPath string
	// Install enos-flight-control into the $HOME directory
	FlightControlUseHomeDir bool
	// What the systemd unit name for the vault service when using systemd for process management.
	SystemdUnitName string
	// How to get k8s pod information.
	*kubernetes.ListPodsRequest
	ListPodOpts []kubernetes.ListPodsRequestOpt
}

// StateRequestOpt is a functional option for a config create request.
type StateRequestOpt func(*StateRequest) *StateRequest

// NewState returns a new instance of Vault's state.
func NewState() *State {
	return &State{}
}

// NewStateRequest takes functional options and returns a new
// systemd unit request.
func NewStateRequest(opts ...StateRequestOpt) *StateRequest {
	c := &StateRequest{
		CLIRequest:        &CLIRequest{},
		FlightControlPath: remoteflight.DefaultFlightControlPath,
		SystemdUnitName:   "vault",
		ListPodsRequest: kubernetes.NewListPodsRequest(
			kubernetes.WithListPodsRequestRetryOpts(
				retry.WithMaxRetries(0),
			),
		),
	}

	for _, opt := range opts {
		c = opt(c)
	}

	for _, opt := range c.ListPodOpts {
		opt(c.ListPodsRequest)
	}

	return c
}

// WithStateRequestBinPath sets the vault binary path.
func WithStateRequestBinPath(path string) StateRequestOpt {
	return func(u *StateRequest) *StateRequest {
		u.CLIRequest.BinPath = path
		return u
	}
}

// WithStateRequestVaultAddr sets the vault address.
func WithStateRequestVaultAddr(addr string) StateRequestOpt {
	return func(u *StateRequest) *StateRequest {
		u.CLIRequest.VaultAddr = addr
		return u
	}
}

// WithStateRequestVaultToken sets the vault token.
func WithStateRequestVaultToken(token string) StateRequestOpt {
	return func(u *StateRequest) *StateRequest {
		u.CLIRequest.Token = token
		return u
	}
}

// WithStateRequestSystemdUnitName sets the vault systemd unit name.
func WithStateRequestSystemdUnitName(unit string) StateRequestOpt {
	return func(u *StateRequest) *StateRequest {
		u.SystemdUnitName = unit
		return u
	}
}

// WithStateRequestFlightControlPath sets the enos-flight-control binary path.
func WithStateRequestFlightControlPath(path string) StateRequestOpt {
	return func(u *StateRequest) *StateRequest {
		u.FlightControlPath = path
		return u
	}
}

// WithStateRequestFlightControlUseHomeDir configures the state request to install
// enos-flight-control into the $HOME directory.
func WithStateRequestFlightControlUseHomeDir() StateRequestOpt {
	return func(u *StateRequest) *StateRequest {
		u.FlightControlUseHomeDir = true
		return u
	}
}

// WithStateRequestListPodsRequestOpts configures the ListPodsRequest with ListPodsRequestOpts.
func WithStateRequestListPodsRequestOpts(opts ...kubernetes.ListPodsRequestOpt) StateRequestOpt {
	return func(u *StateRequest) *StateRequest {
		u.ListPodOpts = opts
		return u
	}
}

// GetState attempts to get the state of the vault cluster and the target node.
//
//nolint:gocyclo,cyclop// this is a complex func because vault has a lot of state and we have to support multiple different process managers.
func GetState(ctx context.Context, tr it.Transport, req *StateRequest) (*State, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("getting vault state: %w", ctx.Err())
	default:
	}

	var err error
	state := NewState()
	// Don't auto-retry target requests here. WaitForState() can handle retrying for us if we need
	// to be retried.
	targetReq := remoteflight.NewTargetRequest(
		remoteflight.WithTargetRequestRetryOpts(
			retry.WithMaxRetries(0),
		),
	)

	pidManager, err := remoteflight.TargetProcessManager(ctx, tr, targetReq)
	if err != nil {
		return state, fmt.Errorf("getting vault state: unable to determine target process manager: %w", err)
	}

	switch pidManager {
	case "systemd":
		// Right now we assume anything using the ssh transport is a linux machine running systemd.
		// Get the systemd unit properties
		sysd := systemd.NewClient(tr, log.NewLogger(ctx))
		if err != nil {
			return nil, fmt.Errorf("getting the systemd unit properties: %w", err)
		}

		state.UnitProperties, err = sysd.ShowProperties(ctx, req.SystemdUnitName)
		if err != nil {
			return state, fmt.Errorf("getting the systemd unit properties: %w", err)
		}
	case "kubernetes":
		k, ok := tr.(*k8s.Transport)
		if !ok {
			return state, errors.New("getting the kubernetes pods state: type mismatch between transport")
		}

		req.ListPodsRequest.Namespace = k.Namespace
		state.PodList, err = k.Client.ListPods(ctx, req.ListPodsRequest)
		if err != nil {
			return state, fmt.Errorf("getting the kubernetes pod information: %w", err)
		}
	default:
	}

	state.Status, err = GetStatus(ctx, tr, req.CLIRequest)
	if err != nil {
		return state, err
	}

	// We use enos-flight-control to read data from the vault /v1/sys/health and /v1/sys/seal-status
	// endpoints.
	opts := []remoteflight.InstallFlightControlOpt{
		remoteflight.WithInstallFlightControlRequestTargetRequest(targetReq),
	}

	if req.FlightControlUseHomeDir {
		opts = append(opts, remoteflight.WithInstallFlightControlRequestUseHomeDir())
	} else {
		opts = append(opts, remoteflight.WithInstallFlightControlRequestPath(req.FlightControlPath))
	}

	fcRes, err := remoteflight.InstallFlightControl(ctx, tr, remoteflight.NewInstallFlightControlRequest(opts...))
	if err != nil {
		return state, fmt.Errorf("getting vault state: failed to install enos-flight-control binary: %w", err)
	}

	state.SealStatus, err = GetSealStatus(ctx, tr, NewSealStatusRequest(
		WithSealStatusRequestVaultAddr(req.VaultAddr),
		WithSealStatusFlightControlPath(fcRes.Path),
	))
	if err != nil {
		return state, err
	}

	state.Health, err = GetHealth(ctx, tr, NewHealthRequest(
		WithHealthRequestVaultAddr(req.VaultAddr),
		WithHealthFlightControlPath(fcRes.Path),
	))
	if err != nil {
		return state, err
	}

	// The following state endpoints require the cluster to be initialized and
	// unsealed.
	initialized, err := state.IsInitialized()
	if err != nil {
		return state, err
	}

	if !initialized {
		return state, nil
	}

	sealed, err := state.IsSealed()
	if err != nil {
		return state, err
	}

	if sealed {
		return state, nil
	}

	replicationEnabled, err := state.ReplicationEnabled()
	if err != nil {
		return state, err
	}

	// Get our replication status if our node has replication enabled and only if the node is active.
	// If the node is unsealed but in standby mode we'll get 500's from the replication API.
	if replicationEnabled && state.Health.StatusIsOneOf(
		HealthStatusInitializedUnsealedActive,
		HealthStatusDRReplicationSecondaryActive,
	) {
		state.ReplicationStatus, err = GetReplicationStatus(ctx, tr, NewReplicationRequest(
			WithReplicationRequestBinPath(req.BinPath),
			WithReplicationRequestVaultAddr(req.VaultAddr),
		))
		if err != nil {
			return state, err
		}
	}

	// The following sub-state endpoints require privileged access, which means
	// that our cluster must be initialized and unsealed and that we have been
	// configured with a token.
	if req.CLIRequest.Token == "" {
		return state, nil
	}

	state.ConfigSanitized, err = GetConfigStateSanitized(ctx, tr, req.CLIRequest)
	if err != nil {
		return state, err
	}

	state.HostInfo, err = GetHostInfo(ctx, tr, req.CLIRequest)
	if err != nil {
		return state, err
	}

	haEnabled, err := state.HAEnabled()
	if err != nil {
		return state, err
	}

	if haEnabled {
		state.HAStatus, err = GetHAStatus(ctx, tr, req.CLIRequest)
		if err != nil {
			return state, err
		}
	}

	storageType, err := state.StorageType()
	if err != nil {
		return state, err
	}

	if storageType == "raft" {
		state.RaftConfig, err = GetRaftConfiguration(ctx, tr, req.CLIRequest)
		if err != nil {
			return state, err
		}

		state.AutopilotConfig, err = GetRaftAutopilotConfiguration(ctx, tr, req.CLIRequest)
		if err != nil {
			return state, err
		}

		state.AutopilotState, err = GetRaftAutopilotState(ctx, tr, req.CLIRequest)
		if err != nil {
			return state, err
		}
	}

	return state, err
}

// WaitForState waits until the vault cluster node state satisfies all of the
// provided checks.
func WaitForState(ctx context.Context, tr it.Transport, req *StateRequest, checks ...CheckStater) (*State, error) {
	checkState := func(ctx context.Context) (any, error) {
		var err error

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		state, err := GetState(ctx, tr, req)
		if err != nil {
			return state, err
		}

		if state == nil {
			return nil, errors.Join(err, errors.New("no state data found"))
		}

		for _, check := range checks {
			err = check(state)
			if err != nil {
				return state, err
			}
		}

		return state, nil
	}

	r, err := retry.NewRetrier(
		retry.WithIntervalFunc(retry.IntervalDuration(5*time.Second)),
		retry.WithRetrierFunc(checkState),
	)

	var state any
	if err == nil {
		state, err = retry.Retry(ctx, r)
	}

	if err != nil {
		err = fmt.Errorf("waiting for vault to enter desired state: %w", err)
	}

	if state == nil {
		return nil, err
	}

	return state.(*State), err
}

// String returns the Vault cluster state as a string.
func (s *State) String() string {
	out := new(strings.Builder)

	s.printStateField(out, s.AutopilotConfig, "Autopilot Config")
	s.printStateField(out, s.AutopilotState, "Autopilot State")
	s.printStateField(out, s.ConfigSanitized, "Configuration")
	s.printStateField(out, s.Health, "Health")
	s.printStateField(out, s.HAStatus, "HA Status")
	s.printStateField(out, s.HostInfo, "Host Info")
	s.printStateField(out, s.ReplicationStatus, "Replication Status")
	s.printStateField(out, s.SealStatus, "Seal Status")
	s.printStateField(out, s.Status, "Status")
	s.printStateField(out, s.PodList, "Kubernetes Pods")

	if s.UnitProperties != nil {
		// Most of the time we don't care about all of the systemd unit properties.
		// Try and find our meaningful status properties. If we can't then something
		// strange is afoot and we'll display all of our props.
		props, err := s.UnitProperties.FindProperties(systemd.EnabledAndRunningProperties)
		if err != nil {
			props = s.UnitProperties
		}
		s.printStateField(out, props, "Systemd Unit Properties")
	}

	return out.String()
}

// printStateField takes a writer, a string, and field name and writes the
// body of the stringer result indented.
func (s *State) printStateField(w io.Writer, f fmt.Stringer, fn string) {
	if f == nil || w == nil {
		return
	}

	fs := f.String()
	if fs == "" {
		return
	}
	_, _ = w.Write([]byte(fmt.Sprintf("%s: \n%s\n", fn, istrings.Indent("  ", fs))))
}

// IsSealed checks whether or not the state is sealed. If we are unable to
// determine the seal status, or the exit code and status body diverge, an
// error will be returned.
func (s *State) IsSealed() (bool, error) {
	if s == nil {
		return true, errors.New("state is unknown")
	}

	if s.Status == nil {
		return true, errors.New("state does not include 'vault status' response")
	}

	if s.Health == nil {
		return false, errors.New("state has no /v1/sys/health data")
	}

	if s.SealStatus == nil {
		return false, errors.New("state has no /v1/sys/seal-status data")
	}

	statusSealed, err := s.Status.IsSealed()
	if err != nil {
		return true, err
	}

	sealStatusSealed, err := s.SealStatus.IsSealed()
	if err != nil {
		return true, err
	}

	healthSealed, err := s.Health.IsSealed()
	if err != nil {
		return true, err
	}

	// Make sure our endpoints agree about being sealed
	if (statusSealed != healthSealed) || (statusSealed != sealStatusSealed) {
		return true, fmt.Errorf(
			"vault status sealed: %t, /v1/sys/health sealed: %t, /v1/sys/seal-status sealed: %t do not agree",
			statusSealed, healthSealed, sealStatusSealed,
		)
	}

	if statusSealed {
		return true, nil
	}

	return false, nil
}

// IsInitialized checks whether or not the state is initialized. If we are
// unable to determine the init status, or the status and health APIs diverge,
// an error will be returned.
func (s *State) IsInitialized() (bool, error) {
	if s == nil {
		return false, errors.New("state is unknown")
	}

	if s.Status == nil {
		return true, errors.New("state does not include 'vault status' response")
	}

	if s.Health == nil {
		return false, errors.New("state has no /v1/sys/health data")
	}

	// Make sure our endpoints agree about being initialized
	if s.Status.Initialized != s.Health.Initialized {
		return false, fmt.Errorf(
			"vault status initialized: %t and /v1/sys/health initialized: %t do not agree",
			s.Status.Initialized, s.Health.Initialized,
		)
	}

	return s.Status.Initialized, nil
}

// ReplicationEnabled checks whether or not the state includes replication health information and if
// replication is enabled.
func (s *State) ReplicationEnabled() (bool, error) {
	if s == nil {
		return false, errors.New("state is unknown")
	}

	if s.Health == nil {
		return false, errors.New("state has no /v1/sys/health data")
	}

	if s.Health.ReplicationDRMode != ReplicationModeUnset &&
		s.Health.ReplicationDRMode != ReplicationModeUnknown &&
		s.Health.ReplicationDRMode != ReplicationModeDisabled {
		return true, nil
	}

	if s.Health.ReplicationPerformanceMode != ReplicationModeUnset &&
		s.Health.ReplicationPerformanceMode != ReplicationModeUnknown &&
		s.Health.ReplicationPerformanceMode != ReplicationModeDisabled {
		return true, nil
	}

	return false, nil
}

// HAEnabled checks whether or not the state includes status infroatmion and if HA is enabled.
func (s *State) HAEnabled() (bool, error) {
	if s == nil {
		return false, errors.New("state is unknown")
	}

	if s.Status == nil {
		return true, errors.New("state does not include 'vault status' response")
	}

	return s.Status.HAEnabled, nil
}

// StorageType gets the storage type from the seal status data.
func (s *State) StorageType() (string, error) {
	if s == nil {
		return "", errors.New("state is unknown")
	}

	if s.SealStatus == nil {
		return "", errors.New("state has no /v1/sys/seal-status response data")
	}

	if s.SealStatus.Data == nil {
		return "", errors.New("state has no /v1/sys/seal-status data")
	}

	return s.SealStatus.Data.StorageType, nil
}

// StatusCode gets the status code from the 'vault status' response.
func (s *State) StatusCode() (StatusCode, error) {
	if s == nil {
		return StatusUnknown, errors.New("state is unknown")
	}

	if s.Status == nil {
		return StatusUnknown, errors.New("state does not include 'vault status' response")
	}

	return s.Status.StatusCode, nil
}
