// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package remoteflight

import (
	"context"
	"errors"
	"fmt"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// CreateDirectoryRequest creates a directory on remote host.
type CreateDirectoryRequest struct {
	DirName  string
	DirOwner string
}

// CreateDirectoryRequestOpt is a functional option for creating directory.
type CreateDirectoryRequestOpt func(*CreateDirectoryRequest) *CreateDirectoryRequest

// NewCreateDirectoryRequest takes functional options and returns a new directory.
func NewCreateDirectoryRequest(opts ...CreateDirectoryRequestOpt) *CreateDirectoryRequest {
	cdir := &CreateDirectoryRequest{}

	for _, opt := range opts {
		cdir = opt(cdir)
	}

	return cdir
}

// WithDirName sets directory name.
func WithDirName(directory string) CreateDirectoryRequestOpt {
	return func(cdir *CreateDirectoryRequest) *CreateDirectoryRequest {
		cdir.DirName = directory

		return cdir
	}
}

// WithDirChown sets directory name.
func WithDirChown(owner string) CreateDirectoryRequestOpt {
	return func(cdir *CreateDirectoryRequest) *CreateDirectoryRequest {
		cdir.DirOwner = owner

		return cdir
	}
}

// CreateDirectory creates the directory and sets owner permissions.
func CreateDirectory(ctx context.Context, tr it.Transport, dir *CreateDirectoryRequest) error {
	if dir == nil {
		return errors.New("no directory or owner provided")
	}

	if dir.DirName == "" {
		return errors.New("no directory provided")
	}

	var err error
	var stdout string
	var stderr string

	stdout, stderr, err = tr.Run(ctx, command.New(fmt.Sprintf(`mkdir -p '%[1]s' || sudo mkdir -p '%[1]s'`, dir.DirName)))
	if err != nil {
		return WrapErrorWith(err, stdout, stderr, "creating directory on target host")
	}

	if dir.DirOwner != "" {
		stdout, stderr, err = tr.Run(ctx, command.New(fmt.Sprintf("chown -R %[1]s %[2]s || sudo chown -R %[1]s %[2]s", dir.DirOwner, dir.DirName)))
		if err != nil {
			return WrapErrorWith(err, stdout, stderr, "changing file ownership")
		}
	}

	return nil
}
