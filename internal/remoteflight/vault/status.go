package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// SealStatus the seal status for Vault
type SealStatus int

// InitStatus the init status for Vault
type InitStatus int

// Vault status exit codes
const (
	UnSealed      SealStatus = 0
	Error         SealStatus = 1
	Sealed        SealStatus = 2
	StatusUnknown SealStatus = 9

	// Inactive - Vault not running
	Inactive InitStatus = 0
	// Uninitialized - Vault active and uninitialized
	Uninitialized InitStatus = 1
	// Initialized - Vault active and initialized
	Initialized InitStatus = 2
)

func (s SealStatus) String() string {
	switch s {
	case UnSealed:
		return "Unsealed"
	case Error:
		return "Error"
	case Sealed:
		return "Sealed"
	case StatusUnknown:
		return "StatusUnknown"
	}
	return "StatusUnknown"
}

func (i InitStatus) String() string {
	switch i {
	case Inactive:
		return "Inactive"
	case Uninitialized:
		return "Uninitialized"
	case Initialized:
		return "Initialized"
	}
	return "StatusUnknown"
}

// State contains the information output from the vault status command
type State struct {
	SealType    string `json:"type"`
	Initialized bool   `json:"initialized"`
	Sealed      bool   `json:"sealed"`
	Version     string `json:"version"`
	HAEnabled   bool   `json:"ha_enabled"`

	// redundant, but useful for polling on status
	SealStatus SealStatus
	InitStatus InitStatus
}

func NewState() State {
	return State{
		Sealed:      true,
		Initialized: false,
		SealStatus:  StatusUnknown,
		InitStatus:  Inactive,
	}
}

type StateCheck func(s State) bool

func CheckIsUnsealed() StateCheck {
	return func(s State) bool {
		return !s.Sealed
	}
}

func CheckIsActive() StateCheck {
	return func(s State) bool {
		return s.InitStatus != Inactive
	}
}

func CheckIsInitialized() StateCheck {
	return func(s State) bool {
		return s.Initialized
	}
}

// CheckSealStatusKnown returns true if the seal status is either Sealed or Unsealed
func CheckSealStatusKnown() StateCheck {
	return func(s State) bool {
		return s.SealStatus != StatusUnknown
	}
}

// StatusRequest is a vault status request
type StatusRequest struct {
	*CLIRequest
}

// StatusRequestOpt is a functional option for a config create request
type StatusRequestOpt func(*StatusRequest) *StatusRequest

// NewStatusRequest takes functional options and returns a new
// systemd unit request
func NewStatusRequest(opts ...StatusRequestOpt) *StatusRequest {
	c := &StatusRequest{
		&CLIRequest{},
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithStatusRequestBinPath sets the vault binary path
func WithStatusRequestBinPath(path string) StatusRequestOpt {
	return func(u *StatusRequest) *StatusRequest {
		u.BinPath = path
		return u
	}
}

// WithStatusRequestVaultAddr sets the vault address
func WithStatusRequestVaultAddr(addr string) StatusRequestOpt {
	return func(u *StatusRequest) *StatusRequest {
		u.VaultAddr = addr
		return u
	}
}

// GetState returns the vault state
func GetState(ctx context.Context, ssh it.Transport, req *StatusRequest) (State, error) {
	state := NewState()
	if req.BinPath == "" {
		return state, fmt.Errorf("you must supply a vault bin path")
	}
	if req.VaultAddr == "" {
		return state, fmt.Errorf("you must supply a vault listen address")
	}

	stdout, stderr, err := ssh.Run(ctx, command.New(
		fmt.Sprintf("%s status -format=json", req.BinPath),
		command.WithEnvVar("VAULT_ADDR", req.VaultAddr),
	))
	// If we get an error check the error code and get the seal status
	if err != nil {
		var exitError *it.ExecError
		if errors.As(err, &exitError) {
			state.SealStatus = SealStatus(exitError.ExitCode())
		} else {
			return state, remoteflight.WrapErrorWith(fmt.Errorf("failed to execute status command, due to: %w", err), stderr)
		}
	} else {
		// if there's no error the exec returned with a zero exit code, therefore the state is unsealed
		state.SealStatus = UnSealed
		state.InitStatus = Initialized
	}

	err = json.Unmarshal([]byte(stdout), &state)
	if err != nil {
		tflog.Error(ctx, "Failed to unmarshal seal status", map[string]interface{}{"state": stdout})
		return state, remoteflight.WrapErrorWith(fmt.Errorf("failed to unmarshal the seal status, due to: %w", err), stderr)
	}
	if state.Initialized {
		state.InitStatus = Initialized
	} else {
		state.InitStatus = Uninitialized
	}

	return state, nil
}

// WaitForState waits until the vault service state satisfies all of the provided StateCheck(s).
// If the context has a duration we will keep trying until it is done.
func WaitForState(ctx context.Context, ssh it.Transport, req *StatusRequest, checks ...StateCheck) (State, error) {
	state := NewState()
	if len(checks) == 0 {
		return state, nil
	}

	var err error
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return state, fmt.Errorf("timed out waiting for vault: %w: state: %+v", ctx.Err(), state)
		case <-ticker.C:
			state, err = GetState(ctx, ssh, req)
			tflog.Debug(ctx, "Checking Vault state", map[string]interface{}{
				"instance": req.VaultAddr,
				"state":    fmt.Sprintf("%+v", state),
			})
			if err == nil {
				checksOk := false
				for _, check := range checks {
					checksOk = checksOk || check(state)
					if checksOk {
						break
					}
				}
				if !checksOk {
					continue
				}
				tflog.Debug(ctx, "Vault state check done", map[string]interface{}{
					"instance": req.VaultAddr,
					"state":    fmt.Sprintf("%+v", state),
				})
				return state, nil
			} else {
				tflog.Error(ctx, "status check failed", map[string]interface{}{"error": err.Error()})
			}
		}
	}
}
