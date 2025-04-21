// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
)

// HostInfoResponse is the JSON stdout result of /v1/sys/host-info.
type HostInfoResponse struct {
	Data *HostInfoData `json:"data,omitempty"`
}

// HostInfoData is the data section of the host-info response.
type HostInfoData struct {
	Host *HostInfoHost `json:"host,omitempty"`
}

// HostInfoHost is the host section of the host-info response.
type HostInfoHost struct {
	BootTime             json.Number `json:"bootTime,omitempty"`
	HostID               string      `json:"hostid,omitempty"`
	Hostname             string      `json:"hostname,omitempty"`
	KernelArch           string      `json:"kernelArch,omitempty"`
	KernelVersion        string      `json:"kernelVersion,omitempty"`
	OS                   string      `json:"os,omitempty"`
	Platform             string      `json:"platform,omitempty"`
	PlatformFamily       string      `json:"platformFamily,omitempty"`
	PlatformVersion      string      `json:"platformVersion,omitempty"`
	Procs                json.Number `json:"procs,omitempty"`
	Uptime               json.Number `json:"uptime,omitempty"`
	VirtualizationRole   string      `json:"virtualizationRole,omitempty"`
	VirtualizationSystem string      `json:"virtualizationSystem,omitempty"`
}

// GetHostInfo returns the vault host info.
func GetHostInfo(ctx context.Context, tr it.Transport, req *CLIRequest) (*HostInfoResponse, error) {
	var err error
	res := NewHostInfoResponse()

	select {
	case <-ctx.Done():
		err = ctx.Err()
	default:
	}

	if req.BinPath == "" {
		err = errors.Join(err, errors.New("you must supply a vault bin path"))
	}

	if req.VaultAddr == "" {
		err = errors.Join(err, errors.New("you must supply a vault listen address"))
	}

	if req.Token == "" {
		err = errors.Join(err, errors.New("you must supply a vault token for the /v1/sys/host-info endpoint"))
	}

	if err == nil {
		stdout, stderr, err1 := tr.Run(ctx, command.New(
			req.BinPath+" read -format=json sys/host-info",
			command.WithEnvVar("VAULT_ADDR", req.VaultAddr),
			command.WithEnvVar("VAULT_TOKEN", req.Token),
		))
		if err1 != nil {
			err = err1
		}
		if stderr != "" {
			err = errors.Join(err, fmt.Errorf("unexpected write to STDERR: %s", stderr))
		}

		// Deserialize the body onto our response.
		if stdout == "" {
			err = errors.Join(err, errors.New("no JSON body was written to STDOUT"))
		} else {
			err = errors.Join(err, json.Unmarshal([]byte(stdout), res))
		}
	}

	if err != nil {
		return nil, errors.Join(errors.New("get vault host info: vault read sys/host-info"), err)
	}

	return res, nil
}

// String returns the host info as a string.
func (s *HostInfoResponse) String() string {
	if s == nil || s.Data == nil {
		return ""
	}

	return s.Data.String()
}

// String returns the host info data as a string.
func (s *HostInfoData) String() string {
	if s == nil || s.Host == nil {
		return ""
	}

	return s.Host.String()
}

// String returns the host info host as a string.
func (s *HostInfoHost) String() string {
	if s == nil {
		return ""
	}

	out := new(strings.Builder)
	_, _ = fmt.Fprintf(out, "Boot Time: %s\n", s.BootTime)
	_, _ = fmt.Fprintf(out, "Host ID: %s\n", s.HostID)
	_, _ = fmt.Fprintf(out, "Hostname: %s\n", s.Hostname)
	_, _ = fmt.Fprintf(out, "Kernel Arch: %s\n", s.KernelArch)
	_, _ = fmt.Fprintf(out, "Kernel Version: %s\n", s.KernelVersion)
	_, _ = fmt.Fprintf(out, "OS: %s\n", s.OS)
	_, _ = fmt.Fprintf(out, "Platform: %s\n", s.Platform)
	_, _ = fmt.Fprintf(out, "Platform Family: %s\n", s.PlatformFamily)
	_, _ = fmt.Fprintf(out, "Procs: %s\n", s.Procs)
	_, _ = fmt.Fprintf(out, "Uptime: %s\n", s.Uptime)

	if s.VirtualizationRole != "" {
		_, _ = fmt.Fprintf(out, "Virtualization Role: %s\n", s.VirtualizationRole)
	}
	if s.VirtualizationSystem != "" {
		_, _ = fmt.Fprintf(out, "Virtualization System: %s\n", s.VirtualizationSystem)
	}

	return out.String()
}

// NewHostInfoResponse returns a new instance of HostInfoResponse.
func NewHostInfoResponse() *HostInfoResponse {
	return &HostInfoResponse{Data: &HostInfoData{Host: &HostInfoHost{}}}
}
