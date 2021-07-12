package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/enos-provider/internal/flightcontrol/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// InitRequest is the init request
type InitRequest struct {
	*CLIRequest
	*InitArguments
}

// InitArguments are the possible arguments to pass to the init command
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

// InitResponse is the init response
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
}

// InitRequestOpt is a functional option for a config create request
type InitRequestOpt func(*InitRequest) *InitRequest

// NewInitRequest takes functional options and returns a new
// systemd unit request
func NewInitRequest(opts ...InitRequestOpt) *InitRequest {
	c := &InitRequest{
		&CLIRequest{},
		&InitArguments{
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

	return c
}

// WithInitRequestBinPath sets the vault binary path
func WithInitRequestBinPath(path string) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.BinPath = path
		return u
	}
}

// WithInitRequestVaultAddr sets the vault address
func WithInitRequestVaultAddr(addr string) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.VaultAddr = addr
		return u
	}
}

// WithInitRequestKeyShares sets the init request key shares
func WithInitRequestKeyShares(shares int) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.KeyShares = shares
		return u
	}
}

// WithInitRequestKeyThreshold sets the init key request threshold
func WithInitRequestKeyThreshold(thres int) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.KeyThreshold = thres
		return u
	}
}

// WithInitRequestPGPKeys sets the init pgp keys
func WithInitRequestPGPKeys(keys []string) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.PGPKeys = keys
		return u
	}
}

// WithInitRequestRecoveryShares sets the init recovery shares
func WithInitRequestRecoveryShares(shares int) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.RecoveryShares = shares
		return u
	}
}

// WithInitRequestRecoveryThreshold sets the init recovery threshold
func WithInitRequestRecoveryThreshold(thres int) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.RecoveryThreshold = thres
		return u
	}
}

// WithInitRequestRecoveryPGPKeys sets the recovery pgp keys
func WithInitRequestRecoveryPGPKeys(keys []string) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.RecoveryPGPKeys = keys
		return u
	}
}

// WithInitRequestRootTokenPGPKey sets the root token pgp key
func WithInitRequestRootTokenPGPKey(key string) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.RootTokenPGPKey = key
		return u
	}
}

// WithInitRequestConsulAuto enables consul service discovery mode
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

// WithInitRequestStoredShares sets the request stored shares
func WithInitRequestStoredShares(shares int) InitRequestOpt {
	return func(u *InitRequest) *InitRequest {
		u.StoredShares = shares
		return u
	}
}

// Validate validates that the init requests has required fields
func (r *InitRequest) Validate() error {
	if r.BinPath == "" {
		return fmt.Errorf("no binary path has been supplied")
	}

	if r.VaultAddr == "" {
		return fmt.Errorf("to vault address has been supplied")
	}

	if r.StoredShares != -1 {
		if r.StoredShares != r.KeyShares {
			return fmt.Errorf("the number of stored shares must be equal to key shares")
		}
	}

	return nil
}

// String returns the init request as an init command
func (r *InitRequest) String() string {
	cmd := &strings.Builder{}
	cmd.WriteString(fmt.Sprintf("%s operator init -format=json", r.BinPath))
	if r.KeyShares != -1 {
		cmd.WriteString(fmt.Sprintf(" -key-shares=%d", r.KeyShares))
	}
	if r.KeyThreshold != -1 {
		cmd.WriteString(fmt.Sprintf(" -key-threshold=%d", r.KeyThreshold))
	}
	if len(r.PGPKeys) > 0 {
		cmd.WriteString(fmt.Sprintf(" -pgp-keys='%s'", strings.Join(r.PGPKeys, ",")))
	}
	if r.RecoveryShares != -1 {
		cmd.WriteString(fmt.Sprintf(" -recovery-shares=%d", r.RecoveryShares))
	}
	if r.RecoveryThreshold != -1 {
		cmd.WriteString(fmt.Sprintf(" -recovery-threshold=%d", r.RecoveryThreshold))
	}
	if len(r.RecoveryPGPKeys) > 0 {
		cmd.WriteString(fmt.Sprintf(" -recovery-pgp-keys='%s'", strings.Join(r.RecoveryPGPKeys, ",")))
	}
	if r.RootTokenPGPKey != "" {
		cmd.WriteString(fmt.Sprintf(" -root-token-pgp-key='%s'", r.RootTokenPGPKey))
	}
	if r.ConsulAuto {
		cmd.WriteString(" -consul-auto=true")
	}
	if r.ConsulService != "" {
		cmd.WriteString(fmt.Sprintf(" -consul-service='%s'", r.ConsulService))
	}
	if r.StoredShares != -1 {
		cmd.WriteString(fmt.Sprintf(" -stored-shares=%d", r.StoredShares))
	}

	return cmd.String()
}

// Init Initializes a vault cluster
func Init(ctx context.Context, ssh it.Transport, req *InitRequest) (*InitResponse, error) {
	res := &InitResponse{}

	stdout, stderr, err := ssh.Run(ctx, command.New(
		req.String(),
		command.WithEnvVar("VAULT_ADDR", req.VaultAddr),
	))
	if err != nil {
		return res, remoteflight.WrapErrorWith(err, stderr)
	}

	if stdout == "" {
		return res, fmt.Errorf("vault initialization command failed to return output")
	}

	err = json.Unmarshal([]byte(stdout), &res)
	if err != nil {
		return res, err
	}

	return res, nil
}
