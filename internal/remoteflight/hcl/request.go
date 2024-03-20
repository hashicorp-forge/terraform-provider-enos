// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"context"
	"errors"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	tfile "github.com/hashicorp-forge/terraform-provider-enos/internal/transport/file"
)

// CreateHCLConfigFileRequest is an HCL config create request.
type CreateHCLConfigFileRequest struct {
	HCLConfig *Builder
	FilePath  string
	Chmod     string
	Chown     string
}

// CreateHCLConfigFileOpt is a functional option for a config create request.
type CreateHCLConfigFileOpt func(*CreateHCLConfigFileRequest) *CreateHCLConfigFileRequest

// NewCreateHCLConfigFileRequest takes functional options and returns a new
// config file request.
func NewCreateHCLConfigFileRequest(opts ...CreateHCLConfigFileOpt) *CreateHCLConfigFileRequest {
	c := &CreateHCLConfigFileRequest{}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithHCLConfigFilePath sets the config file path.
func WithHCLConfigFilePath(path string) CreateHCLConfigFileOpt {
	return func(u *CreateHCLConfigFileRequest) *CreateHCLConfigFileRequest {
		u.FilePath = path
		return u
	}
}

// WithHCLConfigFile sets the config file to use.
func WithHCLConfigFile(unit *Builder) CreateHCLConfigFileOpt {
	return func(u *CreateHCLConfigFileRequest) *CreateHCLConfigFileRequest {
		u.HCLConfig = unit
		return u
	}
}

// WithHCLConfigChmod sets config file permissions.
func WithHCLConfigChmod(chmod string) CreateHCLConfigFileOpt {
	return func(u *CreateHCLConfigFileRequest) *CreateHCLConfigFileRequest {
		u.Chmod = chmod
		return u
	}
}

// WithHCLConfigChown sets config file ownership.
func WithHCLConfigChown(chown string) CreateHCLConfigFileOpt {
	return func(u *CreateHCLConfigFileRequest) *CreateHCLConfigFileRequest {
		u.Chown = chown
		return u
	}
}

// CreateHCLConfigFile takes a context, transport, and create request and
// creates the config file.
func CreateHCLConfigFile(ctx context.Context, tr it.Transport, req *CreateHCLConfigFileRequest) error {
	hcl, err := req.HCLConfig.BuildHCL()
	if err != nil {
		return err
	}

	if req.FilePath == "" {
		return errors.New("you must provide a config file destination path")
	}

	copyOpts := []remoteflight.CopyFileRequestOpt{
		remoteflight.WithCopyFileContent(tfile.NewReader(hcl)),
		remoteflight.WithCopyFileDestination(req.FilePath),
	}

	if req.Chmod != "" {
		copyOpts = append(copyOpts, remoteflight.WithCopyFileChmod(req.Chmod))
	}

	if req.Chown != "" {
		copyOpts = append(copyOpts, remoteflight.WithCopyFileChown(req.Chown))
	}

	return remoteflight.CopyFile(ctx, tr, remoteflight.NewCopyFileRequest(copyOpts...))
}
