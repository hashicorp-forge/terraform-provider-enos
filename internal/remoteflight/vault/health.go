package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	istrings "github.com/hashicorp/enos-provider/internal/strings"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// HealthStatus is the response code to requests to /v1/sys/health.
type HealthStatus int

const (
	// These health status codes differ from the defaults as outlined here:
	//   https://developer.hashicorp.com/vault/api-docs/system/health
	//
	// When we get the health status of a node/cluster the /v1/sys/health
	// endpoint behaves differently depending the node role (active, standby,
	// perf standby, DR active secondary) and the cluster state
	// (initialized and unsealed). The endpoint will return different HTTP
	// status codes which correspond to node health, as well as a JSON body
	// that contains the data. We can't use 'vault read' for this endpoint
	// because it doesn't allow us to pass parameters, nor does it handle
	// the different status codes the endpoint will return. What we've chosen
	// to do is to use enos-flight-control to "download" the health response
	// to STDOUT and then exit the program with the status code that is
	// returned. This will allow us to ascertain the health status and get the
	// body with a single execution.
	//
	// Because we are going to exit with an code to specify health, we have to
	// choose codes that fall into POSIX and HTTP compliance. We need to support
	// waitid(), which means that in practice any exit code will only ever
	// return the first 8 bits, i.e. an int up to 255. We also need our codes
	// to fall into HTTP compliance, and 200 codes are used for success. We
	// choose to avoid all exisiting reserved codes and stay within our range.
	HealthStatusInitializedUnsealedActive    HealthStatus = 230
	HealthStatusUnsealedStandby              HealthStatus = 231
	HealthStatusDRReplicationSecondaryActive HealthStatus = 232
	HealthStatusPerformanceStandby           HealthStatus = 233
	HealthStatusNotInitialized               HealthStatus = 234
	HealthStatusSealed                       HealthStatus = 235
	// Unknown is our default state and is defined outside of LSB range.
	HealthStatusUnknown HealthStatus = 9
)

// String returns the health status response as a string.
func (s HealthStatus) String() string {
	switch s {
	case HealthStatusInitializedUnsealedActive:
		return "initialized-unsealed-active"
	case HealthStatusUnsealedStandby:
		return "unsealed-standby"
	case HealthStatusDRReplicationSecondaryActive:
		return "dr-replication-secondary-active"
	case HealthStatusPerformanceStandby:
		return "performance-standby"
	case HealthStatusNotInitialized:
		return "not-initialized"
	case HealthStatusSealed:
		return "sealed"
	case HealthStatusUnknown:
		return "unknown"
	default:
		return "undefined"
	}
}

// HealthRequest is a vault /v1/sys/health request.
type HealthRequest struct {
	VaultAddr              string
	FlightControlPath      string
	StandbyOk              bool
	PerfStandbyOk          bool
	ActiveCode             HealthStatus
	StandbyCode            HealthStatus
	DRSecondaryCode        HealthStatus
	PerformanceStandbyCode HealthStatus
	SealedCode             HealthStatus
	UnInitCode             HealthStatus
}

// HealthRequestOpt is a functional option for health requests.
type HealthRequestOpt func(*HealthRequest) *HealthRequest

// HealthResponse is the JSON stdout result of /v1/sys/health.
type HealthResponse struct {
	HealthStatus
	ClusterID                  string                     `json:"cluster_id,omitempty"`
	ClusterName                string                     `json:"cluster_name,omitempty"`
	Initialized                bool                       `json:"initialized,omitempty"`
	LastWAL                    uint64                     `json:"last_wal,omitempty"`
	License                    *HealthResponseDataLicense `json:"license,omitempty"`
	PerformanceStandby         bool                       `json:"performance_standby,omitempty"`
	ReplicationDRMode          string                     `json:"replication_dr_mode,omitempty"`
	ReplicationPerformanceMode string                     `json:"replication_performance_mode,omitempty"`
	Sealed                     bool                       `json:"sealed,omitempty"`
	ServerTimeUTC              uint64                     `json:"server_time_utc,omitempty"`
	Standby                    bool                       `json:"standby,omitempty"`
	Version                    string                     `json:"version,omitempty"`
}

// HealthResponseDataLicense is the data body of the license for /v1/sys/health.
type HealthResponseDataLicense struct {
	ExpiryTime string `json:"expiry_time"`
	State      string `json:"state"`
	Terminated bool   `json:"terminated"`
}

// NewHealthRequest takes functional options and returns a new request.
func NewHealthRequest(opts ...HealthRequestOpt) *HealthRequest {
	c := &HealthRequest{
		FlightControlPath:      remoteflight.DefaultFlightControlPath,
		StandbyOk:              false,
		PerfStandbyOk:          false,
		ActiveCode:             HealthStatusInitializedUnsealedActive,
		StandbyCode:            HealthStatusUnsealedStandby,
		DRSecondaryCode:        HealthStatusDRReplicationSecondaryActive,
		PerformanceStandbyCode: HealthStatusPerformanceStandby,
		SealedCode:             HealthStatusSealed,
		UnInitCode:             HealthStatusNotInitialized,
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithHealthFlightControlPath sets the path to flightcontrol.
func WithHealthFlightControlPath(path string) HealthRequestOpt {
	return func(u *HealthRequest) *HealthRequest {
		u.FlightControlPath = path
		return u
	}
}

// WithHealthRequestVaultAddr sets vault address.
func WithHealthRequestVaultAddr(addr string) HealthRequestOpt {
	return func(u *HealthRequest) *HealthRequest {
		u.VaultAddr = addr
		return u
	}
}

// NewHealthResponse returns a new instance of NewHealthResponse.
func NewHealthResponse() *HealthResponse {
	return &HealthResponse{License: &HealthResponseDataLicense{}}
}

// GetHealth returns the vault node health.
func GetHealth(ctx context.Context, tr it.Transport, req *HealthRequest) (*HealthResponse, error) {
	var err error
	res := NewHealthResponse()
	res.HealthStatus = HealthStatusUnknown

	select {
	case <-ctx.Done():
		err = ctx.Err()
	default:
	}

	if req.FlightControlPath == "" {
		err = errors.Join(err, fmt.Errorf("you must supply an enos-flight-control path"))
	}

	if req.VaultAddr == "" {
		err = errors.Join(err, fmt.Errorf("you must supply a vault listen address"))
	}

	if err == nil {
		stdout, stderr, err1 := tr.Run(ctx, command.New(req.String()))
		if err1 != nil {
			// We exited with a non-zero exit code. Try and get the health state
			// from the exit code. If it isn't valid then return an error.
			code, err1 := healthStatusFromError(err1)
			if err1 != nil {
				err = errors.Join(err, err1)
			} else {
				res.HealthStatus = code
			}

			// Try and decode the STDOUT body
			if stdout == "" {
				err = errors.Join(err, fmt.Errorf("no JSON body was written to STDOUT"))
			} else {
				err = errors.Join(err, json.Unmarshal([]byte(stdout), res))
			}
		} else {
			// We should always get a non-zero error code when talking to the
			// health endpoint.
			err = errors.Join(err, fmt.Errorf("received expected response, stdout: %s, stderr: %s", stdout, stderr))
		}
	}

	if err != nil {
		return nil, errors.Join(fmt.Errorf("get vault health: vault read sys/health"), err)
	}

	return res, nil
}

// String returns the health status request as an enos-flight-control command string.
func (r *HealthRequest) String() string {
	return fmt.Sprintf("%s download --exit-with-status-code --stdout --url '%s/v1/sys/health?standbyok=%t&perfstandbyok=%t&activecode=%d&standbycode=%d&drsecondarycode=%d&performancestandbycode=%d&sealedcode=%d&uninitcode=%d'",
		r.FlightControlPath,
		r.VaultAddr,
		r.StandbyOk,
		r.PerfStandbyOk,
		r.ActiveCode,
		r.StandbyCode,
		r.DRSecondaryCode,
		r.PerformanceStandbyCode,
		r.SealedCode,
		r.UnInitCode,
	)
}

// Status is the response code to requests to /v1/sys/health. As we don't actually
// have access to the header status code because we use "vault read" we have
// to determine it by ourselves.
func (r *HealthResponse) Status() HealthStatus {
	return r.HealthStatus
}

// StatusIsOneOf takes one-or-more statuses and returns wether or not the response
// status matches one of the statuses. If no statuses are passed it will return false.
func (r *HealthResponse) StatusIsOneOf(statuses ...HealthStatus) bool {
	if len(statuses) == 0 {
		return false
	}

	s := r.Status()
	for _, status := range statuses {
		if status == s {
			return true
		}
	}

	return false
}

// IsSealed returns whether or not the node is sealed.
func (r *HealthResponse) IsSealed() (bool, error) {
	if r == nil {
		return true, fmt.Errorf("health status is unknown")
	}

	statusSealed := r.StatusIsOneOf(
		HealthStatusNotInitialized,
		HealthStatusSealed,
	)

	if r.Sealed != statusSealed {
		return true, fmt.Errorf(
			"health status does not match status code, expected sealed=%t, got sealed=%t",
			statusSealed, r.Sealed,
		)
	}

	return statusSealed, nil
}

// String returns the health response as a string.
func (r *HealthResponse) String() string {
	if r == nil {
		return ""
	}

	out := new(strings.Builder)
	_, _ = out.WriteString(fmt.Sprintf("Status: %s\n", r.Status()))
	_, _ = out.WriteString(fmt.Sprintf("Cluster ID: %s\n", r.ClusterID))
	_, _ = out.WriteString(fmt.Sprintf("Cluster Name: %s\n", r.ClusterID))
	_, _ = out.WriteString(fmt.Sprintf("Initialized: %t\n", r.Initialized))
	_, _ = out.WriteString(fmt.Sprintf("Sealed: %t\n", r.Sealed))
	_, _ = out.WriteString(fmt.Sprintf("Standby: %t\n", r.Standby))
	_, _ = out.WriteString(fmt.Sprintf("Version: %t\n", r.Standby))
	_, _ = out.WriteString(fmt.Sprintf("Last WAL: %d\n", r.LastWAL))
	_, _ = out.WriteString(fmt.Sprintf("Performance Standby: %t\n", r.PerformanceStandby))
	_, _ = out.WriteString(fmt.Sprintf("Replication DR Mode: %s\n", r.ReplicationDRMode))
	_, _ = out.WriteString(fmt.Sprintf("Replication Performance Mode: %s\n", r.ReplicationPerformanceMode))
	_, _ = out.WriteString(fmt.Sprintf("Server Time UTC: %d\n", r.ServerTimeUTC))
	_, _ = out.WriteString(fmt.Sprintf("License:\n%s", istrings.Indent("  ", r.License.String())))

	return out.String()
}

// String returns the license health as a string.
func (l *HealthResponseDataLicense) String() string {
	if (l == nil) || (l.ExpiryTime == "" && l.State == "" && !l.Terminated) {
		return ""
	}

	return fmt.Sprintf("State: %s\nExpiry Time: %s\nTerminated: %t\n",
		l.State, l.ExpiryTime, l.Terminated,
	)
}

// healthStatusFromError takes an error message and attempts to return a
// status code if it embeds an error exit code.
func healthStatusFromError(err error) (HealthStatus, error) {
	var exitError *it.ExecError

	if errors.As(err, &exitError) {
		code := HealthStatus(exitError.ExitCode())
		switch code {
		case HealthStatusInitializedUnsealedActive,
			HealthStatusUnsealedStandby,
			HealthStatusDRReplicationSecondaryActive,
			HealthStatusPerformanceStandby,
			HealthStatusNotInitialized,
			HealthStatusSealed:
			return code, nil
		case HealthStatusUnknown:
			return code, fmt.Errorf("invalid health status: %d (%[1]s)", code)
		default:
			return code, fmt.Errorf("invalid health status: %d (%[1]s)", code)
		}
	}

	return HealthStatusUnknown, fmt.Errorf("err did not include exit code: %w", err)
}
