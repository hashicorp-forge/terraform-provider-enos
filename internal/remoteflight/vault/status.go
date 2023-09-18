package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// StatusCode is the exit code of "vault status".
type StatusCode int

const (
	// The exit code of "vault status" reflects our seal status
	// https://developer.hashicorp.com/vault/docs/commands/status
	StatusInitializedUnsealed StatusCode = 0
	StatusError               StatusCode = 1
	StatusSealed              StatusCode = 2
	// Unknown is our default state and is defined outside of LSB range.
	StatusUnknown StatusCode = 9
)

// String returns the status code as a string.
func (s StatusCode) String() string {
	switch s {
	case StatusInitializedUnsealed:
		return "unsealed"
	case StatusError:
		return "error"
	case StatusSealed:
		return "sealed"
	case StatusUnknown:
		return "unknown"
	default:
		return "undefined"
	}
}

// StatusResponse is the JSON stdout result of "vault status". It should be
// taken with a grain of salt. For seal status in particular, always trust the
// exit code before the status response.
type StatusResponse struct {
	StatusCode
	SealType    string `json:"type,omitempty"`
	Initialized bool   `json:"initialized,omitempty"`
	Sealed      bool   `json:"sealed,omitempty"`
	Version     string `json:"version,omitempty"`
	HAEnabled   bool   `json:"ha_enabled,omitempty"`
}

// GetStatus returns the vault node status.
func GetStatus(ctx context.Context, tr it.Transport, req *CLIRequest) (*StatusResponse, error) {
	var err error
	res := &StatusResponse{StatusCode: StatusUnknown}

	select {
	case <-ctx.Done():
		err = ctx.Err()
	default:
	}

	if req.BinPath == "" {
		err = errors.Join(err, fmt.Errorf("you must supply a vault bin path"))
	}

	if req.VaultAddr == "" {
		err = errors.Join(err, fmt.Errorf("you must supply a vault listen address"))
	}

	if err == nil {
		var err1 error
		stdout, stderr, err1 := tr.Run(ctx, command.New(
			fmt.Sprintf("%s status -format=json", req.BinPath),
			command.WithEnvVar("VAULT_ADDR", req.VaultAddr),
		))

		// Set our status from the status command exit code
		var code StatusCode
		var err2 error
		if err1 == nil && stderr == "" {
			code = StatusInitializedUnsealed
		} else if err1 == nil && stderr != "" {
			code = StatusError
		} else {
			code, err2 = vaultStatusCodeFromError(err1)
		}
		res.StatusCode = code

		switch res.StatusCode {
		// Don't set our outer error if our seal status is known.
		case StatusInitializedUnsealed, StatusSealed:
		case StatusError, StatusUnknown:
			err = errors.Join(err, err1, err2)
		default:
			err = errors.Join(err, err1, err2)
		}

		if stderr != "" {
			err = errors.Join(err, fmt.Errorf("unexpected write to STDERR: %s", stderr))
		}

		// Deserialize the body
		if stdout == "" {
			err = errors.Join(err, fmt.Errorf("no JSON body was written to STDOUT"))
		} else {
			err = errors.Join(err, json.Unmarshal([]byte(stdout), res))
		}
	}

	if err != nil {
		return nil, errors.Join(fmt.Errorf("get vault status: vault status"), err)
	}

	return res, err
}

// IsSealed checks whether or not the status of the cluster is sealed. If we
// are unable to determine the seal status, or the exit code and status body
// diverge, an error will be returned.
func (s *StatusResponse) IsSealed() (bool, error) {
	if s == nil {
		return true, fmt.Errorf("state does not include status body")
	}

	switch s.StatusCode {
	case StatusInitializedUnsealed:
		if s.Sealed {
			return true, fmt.Errorf("status response does not match status code, expected sealed=false, got sealed=true")
		}

		return false, nil
	case StatusError:
		return true, fmt.Errorf("unable to determine seal status because status returned an error exit code")
	case StatusSealed:
		if !s.Sealed {
			return true, fmt.Errorf("status response does not match status code, expected sealed=true, got sealed=false")
		}

		return true, nil
	case StatusUnknown:
		return true, fmt.Errorf("unable to determine vault status")
	default:
		return true, fmt.Errorf("unable to determine vault status, unexpected exit code %s", s.StatusCode)
	}
}

// String returns the status response as a string.
func (s *StatusResponse) String() string {
	if s == nil {
		return ""
	}

	out := new(strings.Builder)
	_, _ = out.WriteString(fmt.Sprintf("Code: %[1]d (%[1]s)\n", s.StatusCode))
	if s.SealType != "" {
		_, _ = out.WriteString(fmt.Sprintf("Seal Type: %s\n", s.SealType))
	}
	_, _ = out.WriteString(fmt.Sprintf("Initialized: %t\n", s.Initialized))
	_, _ = out.WriteString(fmt.Sprintf("Sealed: %t\n", s.Sealed))
	if s.Version != "" {
		_, _ = out.WriteString(fmt.Sprintf("Version: %s\n", s.Version))
	}
	_, _ = out.WriteString(fmt.Sprintf("HA Enabled: %t\n", s.HAEnabled))

	return out.String()
}

// NewStatusResponse returns a new instance of StatusResponse.
func NewStatusResponse() *StatusResponse {
	return &StatusResponse{}
}

// vaultStatusCodeFromError takes an error message and attempts to return a
// status code if it embeds an error exit code.
func vaultStatusCodeFromError(err error) (StatusCode, error) {
	var exitError *it.ExecError

	if errors.As(err, &exitError) {
		code := StatusCode(exitError.ExitCode())
		switch code {
		case StatusInitializedUnsealed, StatusSealed, StatusError:
			return code, nil
		case StatusUnknown:
			return code, fmt.Errorf("did not get an exit code: %d (%[1]s)", code)
		default:
			return code, fmt.Errorf("invalid exit code: %d (%[1]s)", code)
		}
	}

	return StatusUnknown, fmt.Errorf("err did not include exit code: %w", err)
}
