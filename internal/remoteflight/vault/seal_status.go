// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// SealStatusRequest is a vault /v1/sys/seal-status request.
type SealStatusRequest struct {
	VaultAddr         string
	FlightControlPath string
}

// SealStatusRequestOpt is a functional option for seal-status requests.
type SealStatusRequestOpt func(*SealStatusRequest) *SealStatusRequest

// SealStatusResponse is the JSON stdout result of "vault status". It should be
// taken with a grain of salt. For seal status in particular, always trust the
// exit code before the status response.
type SealStatusResponse struct {
	Data *SealStatusResponseData `json:"data,omitempty"`
}

// SealStatusResponseData is the seal data in the seal response.
type SealStatusResponseData struct {
	BuildDate    string      `json:"build_date,omitempty"`
	ClusterID    string      `json:"cluster_id,omitempty"`
	ClusterName  string      `json:"cluster_name,omitempty"`
	Initialized  bool        `json:"initialized,omitempty"`
	Migration    bool        `json:"migration,omitempty"`
	Number       json.Number `json:"n,omitempty"`
	Nonce        string      `json:"nonce,omitempty"`
	Progress     json.Number `json:"progress,omitempty"`
	RecoverySeal bool        `json:"recovery_seal,omitempty"`
	Sealed       bool        `json:"sealed,omitempty"`
	StorageType  string      `json:"storage_type,omitempty"`
	Threshold    json.Number `json:"t,omitempty"`
	Type         SealType    `json:"type,omitempty"`
	Version      string      `json:"version,omitempty"`
}

// NewSealStatusRequest takes functional options and returns a new request.
func NewSealStatusRequest(opts ...SealStatusRequestOpt) *SealStatusRequest {
	c := &SealStatusRequest{
		FlightControlPath: remoteflight.DefaultFlightControlPath,
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// NewSealStatusResponse returns a new instance of SealStatusResponse.
func NewSealStatusResponse() *SealStatusResponse {
	return &SealStatusResponse{Data: &SealStatusResponseData{}}
}

// WithSealStatusFlightControlPath sets the path to flightcontrol.
func WithSealStatusFlightControlPath(path string) SealStatusRequestOpt {
	return func(u *SealStatusRequest) *SealStatusRequest {
		u.FlightControlPath = path
		return u
	}
}

// WithSealStatusRequestVaultAddr sets vault address.
func WithSealStatusRequestVaultAddr(addr string) SealStatusRequestOpt {
	return func(u *SealStatusRequest) *SealStatusRequest {
		u.VaultAddr = addr
		return u
	}
}

// GetSealStatus returns the vault node seal status.
func GetSealStatus(ctx context.Context, tr it.Transport, req *SealStatusRequest) (*SealStatusResponse, error) {
	var err error
	res := NewSealStatusResponse()

	select {
	case <-ctx.Done():
		err = ctx.Err()
	default:
	}

	if req.FlightControlPath == "" {
		err = errors.Join(err, errors.New("you must supply an enos-flight-control path"))
	}

	if req.VaultAddr == "" {
		err = errors.Join(err, errors.New("you must supply a vault listen address"))
	}

	if err == nil {
		stdout, stderr, err1 := tr.Run(ctx, command.New(req.String()))
		if err1 != nil {
			err = errors.Join(err, err1)
		}
		if stderr != "" {
			err = errors.Join(err, fmt.Errorf("unexpected write to STDERR: %s", stderr))
		}

		// Deserialize the body onto our response.
		if stdout == "" {
			err = errors.Join(err, errors.New("no JSON body was written to STDOUT"))
		} else {
			err = errors.Join(err, json.Unmarshal([]byte(stdout), res))

			if res.Data == nil || res.Data.Type == "" {
				// Our response body didnt have data. This could be because
				// older versions of vault have a different response body for
				// this API. Unmarshal the body again against the data in the
				// response struct.
				err = errors.Join(err, json.Unmarshal([]byte(stdout), res.Data))
			}
		}
	}

	if err != nil {
		return nil, errors.Join(errors.New("get vault seal status: vault read sys/seal-status"), err)
	}

	return res, nil
}

// String returns the health status request as an enos-flight-control command string.
// We use enos-flight-control here over `vault read` because the response body
// of this API is different among vault versions. At some point in the 1.11.x
// series the API changed to conform to what `vault read` expects, but we have
// get the raw body to support prior and post response body types.
func (r *SealStatusRequest) String() string {
	return fmt.Sprintf("%s download --stdout --url '%s/v1/sys/seal-status'",
		r.FlightControlPath,
		r.VaultAddr,
	)
}

// IsSealed checks whether or not the status of the cluster is sealed.
func (s *SealStatusResponse) IsSealed() (bool, error) {
	if s == nil {
		return true, errors.New("seal response is nil")
	}

	if s.Data == nil {
		return true, errors.New("seal response does not have seal data")
	}

	return s.Data.Sealed, nil
}

// String returns the seal data as a string.
func (s *SealStatusResponse) String() string {
	if s == nil || s.Data == nil {
		return ""
	}

	return s.Data.String()
}

// String returns the seal data as a string.
func (s *SealStatusResponseData) String() string {
	if s == nil {
		return ""
	}

	out := new(strings.Builder)
	_, _ = out.WriteString(fmt.Sprintf("Build Date: %s\n", s.BuildDate))
	_, _ = out.WriteString(fmt.Sprintf("Cluster ID: %s\n", s.ClusterID))
	_, _ = out.WriteString(fmt.Sprintf("Cluster Name: %s\n", s.ClusterName))
	_, _ = out.WriteString(fmt.Sprintf("Initialized: %t\n", s.Initialized))
	_, _ = out.WriteString(fmt.Sprintf("Migration: %t\n", s.Migration))
	_, _ = out.WriteString(fmt.Sprintf("Number: %s\n", s.Number))
	_, _ = out.WriteString(fmt.Sprintf("Nonce: %s\n", s.Nonce))
	_, _ = out.WriteString(fmt.Sprintf("Progress: %s\n", s.Progress))
	_, _ = out.WriteString(fmt.Sprintf("Recovery Seal: %t\n", s.RecoverySeal))
	_, _ = out.WriteString(fmt.Sprintf("Sealed: %t\n", s.Sealed))
	_, _ = out.WriteString(fmt.Sprintf("Storage Type: %s\n", s.StorageType))
	_, _ = out.WriteString(fmt.Sprintf("Threshold: %s\n", s.Threshold))
	_, _ = out.WriteString(fmt.Sprintf("Type: %s\n", s.Type))
	_, _ = out.WriteString(fmt.Sprintf("Version: %s\n", s.Version))

	return out.String()
}
