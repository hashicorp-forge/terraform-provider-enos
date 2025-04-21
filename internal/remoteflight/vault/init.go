// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/k8s"
)

// InitRequest is the init request.
type InitRequest struct {
	*StateRequest
	StateRequestOpts []StateRequestOpt
	*InitArguments
}

// InitArguments are the possible arguments to pass to the init command.
type InitArguments struct {
	KeyShares         int
	KeyThreshold      int
	PGPKeys           []string
	RecoveryShares    int
	RecoveryThreshold int
	RecoveryPGPKeys   []string
	RootTokenPGPKey   string
	ConsulAuto        bool
	ConsulService     string
	StoredShares      int
}

// InitResponse is the init response.
type InitResponse struct {
	UnsealKeysB64         []string    `json:"unseal_keys_b64"`
	UnsealKeysHex         []string    `json:"unseal_keys_hex"`
	UnsealShares          json.Number `json:"unseal_shares"`
	UnsealThreshold       json.Number `json:"unseal_threshold"`
	RecoveryKeysB64       []string    `json:"recovery_keys_b64"`
	RecoveryKeysHex       []string    `json:"recovery_keys_hex"`
	RecoveryKeysShares    json.Number `json:"recovery_keys_shares"`
	RecoveryKeysThreshold json.Number `json:"recovery_keys_threshold"`
	RootToken             string      `json:"root_token"`
	PriorState            *State
	PostState             *State
}

// InitRequestOpt is a functional option for a config create request.
type InitRequestOpt func(*InitRequest) *InitRequest

// NewInitRequest takes functional options and returns a new
// systemd unit request.
func NewInitRequest(opts ...InitRequestOpt) *InitRequest {
	c := &InitRequest{
		StateRequest: NewStateRequest(),
		InitArguments: &InitArguments{
			KeyShares:         -1,
			KeyThreshold:      -1,
			PGPKeys:           []string{},
			RecoveryShares:    -1,
			RecoveryThreshold: -1,
			RecoveryPGPKeys:   []string{},
			StoredShares:      -1,
		},
	}

	for _, opt := range opts {
		c = opt(c)
	}

	for _, opt := range c.StateRequestOpts {
		opt(c.StateRequest)
	}

	return c
}

// WithInitRequestStateRequestOpts sets the options for the state request.
func WithInitRequestStateRequestOpts(opts ...StateRequestOpt) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.StateRequestOpts = opts
		return u
	}
}

// WithInitRequestKeyShares sets the init request key shares.
func WithInitRequestKeyShares(shares int) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.KeyShares = shares
		return u
	}
}

// WithInitRequestKeyThreshold sets the init key request threshold.
func WithInitRequestKeyThreshold(thres int) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.KeyThreshold = thres
		return u
	}
}

// WithInitRequestPGPKeys sets the init pgp keys.
func WithInitRequestPGPKeys(keys []string) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.PGPKeys = keys
		return u
	}
}

// WithInitRequestRecoveryShares sets the init recovery shares.
func WithInitRequestRecoveryShares(shares int) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.RecoveryShares = shares
		return u
	}
}

// WithInitRequestRecoveryThreshold sets the init recovery threshold.
func WithInitRequestRecoveryThreshold(thres int) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.RecoveryThreshold = thres
		return u
	}
}

// WithInitRequestRecoveryPGPKeys sets the recovery pgp keys.
func WithInitRequestRecoveryPGPKeys(keys []string) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.RecoveryPGPKeys = keys
		return u
	}
}

// WithInitRequestRootTokenPGPKey sets the root token pgp key.
func WithInitRequestRootTokenPGPKey(key string) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.RootTokenPGPKey = key
		return u
	}
}

// WithInitRequestConsulAuto enables consul service discovery mode.
func WithInitRequestConsulAuto(auto bool) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.ConsulAuto = auto
		return u
	}
}

// WithInitRequestConsulService sets the service name for consul service discovery
// mode.
func WithInitRequestConsulService(service string) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.ConsulService = service
		return u
	}
}

// WithInitRequestStoredShares sets the request stored shares.
func WithInitRequestStoredShares(shares int) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.StoredShares = shares
		return u
	}
}

// Validate validates that the init requests has required fields.
func (r *InitRequest) Validate() error {
	if r.BinPath == "" {
		return errors.New("no binary path has been supplied")
	}

	if r.VaultAddr == "" {
		return errors.New("to vault address has been supplied")
	}

	if r.StoredShares != -1 {
		if r.StoredShares != r.KeyShares {
			return errors.New("the number of stored shares must be equal to key shares")
		}
	}

	return nil
}

// String returns the init request as an init command.
func (r *InitRequest) String() string {
	if r == nil {
		return ""
	}

	cmd := &strings.Builder{}
	cmd.WriteString(r.BinPath + " operator init -format=json")
	if r.KeyShares != -1 {
		fmt.Fprintf(cmd, " -key-shares=%d", r.KeyShares)
	}
	if r.KeyThreshold != -1 {
		fmt.Fprintf(cmd, " -key-threshold=%d", r.KeyThreshold)
	}
	if len(r.PGPKeys) > 0 {
		fmt.Fprintf(cmd, " -pgp-keys='%s'", strings.Join(r.PGPKeys, ","))
	}
	if r.RecoveryShares != -1 {
		fmt.Fprintf(cmd, " -recovery-shares=%d", r.RecoveryShares)
	}
	if r.RecoveryThreshold != -1 {
		fmt.Fprintf(cmd, " -recovery-threshold=%d", r.RecoveryThreshold)
	}
	if len(r.RecoveryPGPKeys) > 0 {
		fmt.Fprintf(cmd, " -recovery-pgp-keys='%s'", strings.Join(r.RecoveryPGPKeys, ","))
	}
	if r.RootTokenPGPKey != "" {
		fmt.Fprintf(cmd, " -root-token-pgp-key='%s'", r.RootTokenPGPKey)
	}
	if r.ConsulAuto {
		cmd.WriteString(" -consul-auto=true")
	}
	if r.ConsulService != "" {
		fmt.Fprintf(cmd, " -consul-service='%s'", r.ConsulService)
	}
	if r.StoredShares != -1 {
		fmt.Fprintf(cmd, " -stored-shares=%d", r.StoredShares)
	}

	return cmd.String()
}

// Init initializes a vault cluster.
func Init(ctx context.Context, tr it.Transport, req *InitRequest) (*InitResponse, error) {
	binPath := req.BinPath
	if binPath == "" {
		return nil, errors.New("you must supply a vault bin path")
	}

	vaultAddr := req.VaultAddr
	if vaultAddr == "" {
		return nil, errors.New("you must supply a vault listen address")
	}

	var err error
	res := &InitResponse{}

	priorStateChecks := []CheckStater{
		CheckStateSealStateIsKnown(),
	}
	postStateChecks := []CheckStater{
		CheckStateSealStateIsKnown(),
		CheckStateIsInitialized(),
	}

	switch tr.Type() {
	case it.TransportType("ssh"):
		priorStateChecks = append(priorStateChecks, CheckStateHasSystemdEnabledAndRunningProperties())
		postStateChecks = append(postStateChecks, CheckStateHasSystemdEnabledAndRunningProperties())
	case it.TransportType("kubernetes"):
		k, ok := tr.(*k8s.Transport)
		if ok {
			priorStateChecks = append(priorStateChecks, CheckStatePodHasPhase(k.Pod, v1.PodRunning))
			postStateChecks = append(postStateChecks, CheckStatePodHasPhase(k.Pod, v1.PodRunning))
		}
	default:
	}

	// Wait for vault to ready to initialize.
	res.PriorState, err = WaitForState(ctx, tr, req.StateRequest, priorStateChecks...)
	if err != nil {
		return res, fmt.Errorf("waiting for vault cluster to be ready to initialize: %w", err)
	}

	// Check if we're already initialized and move on.
	ok, err := res.PriorState.IsInitialized()
	if err == nil && ok {
		res.PostState, err = WaitForState(ctx, tr, req.StateRequest, postStateChecks...)
		if err != nil {
			return res, fmt.Errorf("waiting for vault cluster to be ready to be initialized: %w", err)
		}

		return res, nil
	}

	// Initialize vault
	tflog.Debug(ctx, "Running Vault Init command: "+req.String())
	stdout, stderr, err := tr.Run(ctx, command.New(
		req.String(),
		command.WithEnvVar("VAULT_ADDR", req.VaultAddr),
	))

	if stdout == "" {
		err = errors.Join(err, errors.New("init command did not return data"))
	}
	if stderr != "" {
		err = errors.Join(err, fmt.Errorf("init command had unexpected write to STDERR: %s", stderr))
	}

	if err == nil {
		err = json.Unmarshal([]byte(stdout), &res)
		if err != nil {
			err = fmt.Errorf("deserialize JSON body of init: %w", err)
		}
		tflog.Debug(ctx, fmt.Sprintf("Vault Init command Response: %#v", res))
	}

	if err != nil {
		// Get our post-init state and return our error
		var err1 error
		res.PostState, err1 = GetState(ctx, tr, req.StateRequest)
		err = errors.Join(err, err1)

		return res, err
	}

	// Wait for vault to be initialized.
	res.PostState, err = WaitForState(ctx, tr, req.StateRequest, postStateChecks...)
	if err != nil {
		return res, fmt.Errorf("waiting for vault cluster to be ready to be initialized: %w", err)
	}

	return res, nil
}
