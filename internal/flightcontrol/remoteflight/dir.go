package remoteflight

import (
	"context"
	"fmt"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// CreateDirRequest creates a directory on remote host
type CreateDirRequest struct {
	DirName  string
	DirOwner string
}

// CreateDirRequestOpt is a functional option for creating directory
type CreateDirRequestOpt func(*CreateDirRequest) *CreateDirRequest

// NewCreateDirRequest takes functional options and returns a new directory
func NewCreateDirRequest(opts ...CreateDirRequestOpt) *CreateDirRequest {
	cdir := &CreateDirRequest{}

	for _, opt := range opts {
		cdir = opt(cdir)
	}
	return cdir
}

// WithDirName sets directory name
func WithDirName(directory string) CreateDirRequestOpt {
	return func(cdir *CreateDirRequest) *CreateDirRequest {
		cdir.DirName = directory
		return cdir
	}
}

// WithDirChown sets directory name
func WithDirChown(owner string) CreateDirRequestOpt {
	return func(cdir *CreateDirRequest) *CreateDirRequest {
		cdir.DirOwner = owner
		return cdir
	}
}

// CreateDir creates the directory and sets owner permissions
func CreateDir(ctx context.Context, ssh it.Transport, dir *CreateDirRequest) error {
	if dir == nil {
		return fmt.Errorf("no directory or owner provided")
	}

	if dir.DirName == "" {
		return fmt.Errorf("no directory provided")
	}

	var err error
	var stdout string
	var stderr string

	stdout, stderr, err = ssh.Run(ctx, command.New(fmt.Sprintf(`sudo mkdir -p '%s'`, dir.DirName)))
	if err != nil {
		return WrapErrorWith(err, stdout, stderr, "creating directory on target host")
	}

	if dir.DirOwner != "" {
		stderr, stdout, err = ssh.Run(ctx, command.New(fmt.Sprintf("sudo chown -R %s %s", dir.DirOwner, dir.DirName)))
		if err != nil {
			return WrapErrorWith(err, stdout, stderr, "changing file ownership")
		}
	}
	return nil
}
