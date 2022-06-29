package remoteflight

import (
	"context"
	"fmt"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// CreateDirectoryRequest creates a directory on remote host
type CreateDirectoryRequest struct {
	DirName  string
	DirOwner string
}

// CreateDirectoryRequestOpt is a functional option for creating directory
type CreateDirectoryRequestOpt func(*CreateDirectoryRequest) *CreateDirectoryRequest

// NewCreateDirectoryRequest takes functional options and returns a new directory
func NewCreateDirectoryRequest(opts ...CreateDirectoryRequestOpt) *CreateDirectoryRequest {
	cdir := &CreateDirectoryRequest{}

	for _, opt := range opts {
		cdir = opt(cdir)
	}
	return cdir
}

// WithDirName sets directory name
func WithDirName(directory string) CreateDirectoryRequestOpt {
	return func(cdir *CreateDirectoryRequest) *CreateDirectoryRequest {
		cdir.DirName = directory
		return cdir
	}
}

// WithDirChown sets directory name
func WithDirChown(owner string) CreateDirectoryRequestOpt {
	return func(cdir *CreateDirectoryRequest) *CreateDirectoryRequest {
		cdir.DirOwner = owner
		return cdir
	}
}

// CreateDirectory creates the directory and sets owner permissions
func CreateDirectory(ctx context.Context, client it.Transport, dir *CreateDirectoryRequest) error {
	if dir == nil {
		return fmt.Errorf("no directory or owner provided")
	}

	if dir.DirName == "" {
		return fmt.Errorf("no directory provided")
	}

	var err error
	var stdout string
	var stderr string

	stdout, stderr, err = client.Run(ctx, command.New(fmt.Sprintf(`sudo mkdir -p '%s'`, dir.DirName)))
	if err != nil {
		return WrapErrorWith(err, stdout, stderr, "creating directory on target host")
	}

	if dir.DirOwner != "" {
		stderr, stdout, err = client.Run(ctx, command.New(fmt.Sprintf("sudo chown -R %s %s", dir.DirOwner, dir.DirName)))
		if err != nil {
			return WrapErrorWith(err, stdout, stderr, "changing file ownership")
		}
	}
	return nil
}
