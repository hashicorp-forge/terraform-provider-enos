package remoteflight

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/hashicorp/enos-provider/internal/random"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// CopyFileRequest copies a file to the remote host
type CopyFileRequest struct {
	Content     it.Copyable
	TmpDir      string
	Chmod       string
	Chown       string
	Destination string
}

// CopyFileRequestOpt is a functional option for file copy
type CopyFileRequestOpt func(*CopyFileRequest) *CopyFileRequest

// NewCopyFileRequest takes functional options and returns a new file copy
func NewCopyFileRequest(opts ...CopyFileRequestOpt) *CopyFileRequest {
	cf := &CopyFileRequest{
		TmpDir: "/tmp",
	}

	for _, opt := range opts {
		cf = opt(cf)
	}

	return cf
}

// WithCopyFileContent sets content to be copied
func WithCopyFileContent(content it.Copyable) CopyFileRequestOpt {
	return func(cf *CopyFileRequest) *CopyFileRequest {
		cf.Content = content
		return cf
	}
}

// WithCopyFileTmpDir sets temporary directory to use
func WithCopyFileTmpDir(dir string) CopyFileRequestOpt {
	return func(cf *CopyFileRequest) *CopyFileRequest {
		cf.TmpDir = dir
		return cf
	}
}

// WithCopyFileChmod sets permissions
func WithCopyFileChmod(chmod string) CopyFileRequestOpt {
	return func(cf *CopyFileRequest) *CopyFileRequest {
		cf.Chmod = chmod
		return cf
	}
}

// WithCopyFileChown sets ownership
func WithCopyFileChown(chown string) CopyFileRequestOpt {
	return func(cf *CopyFileRequest) *CopyFileRequest {
		cf.Chown = chown
		return cf
	}
}

// WithCopyFileDestination sets file destination
func WithCopyFileDestination(destination string) CopyFileRequestOpt {
	return func(cf *CopyFileRequest) *CopyFileRequest {
		cf.Destination = destination
		return cf
	}
}

// CopyFile copies a file to the remote host. It first copies a file to a temporary
// directory, sets permissions, then copies to the destination directory as
// a superuser.
func CopyFile(ctx context.Context, ssh it.Transport, file *CopyFileRequest) error {
	if file == nil {
		return fmt.Errorf("no file copy request provided")
	}

	if file.Destination == "" {
		return fmt.Errorf("you must supply a destination path")
	}

	tmpPath := filepath.Join(file.TmpDir, fmt.Sprintf("%s-%s",
		filepath.Base(file.Destination),
		random.ID(),
	))

	var err error
	var stdout string
	var stderr string

	err = ssh.Copy(ctx, file.Content, tmpPath)
	if err != nil {
		return fmt.Errorf("copying file to target host: %w", err)
	}

	if file.Chmod != "" {
		stderr, stdout, err = ssh.Run(ctx, command.New(fmt.Sprintf("chmod %s %s", file.Chmod, tmpPath)))
		if err != nil {
			return WrapErrorWith(err, stdout, stderr, "changing file permissions")
		}
	}

	if file.Chown != "" {
		stderr, stdout, err = ssh.Run(ctx, command.New(fmt.Sprintf("sudo chown %s %s", file.Chown, tmpPath)))
		if err != nil {
			return WrapErrorWith(err, stdout, stderr, "changing file ownership")
		}
	}

	stdout, stderr, err = ssh.Run(ctx, command.New(fmt.Sprintf(`sudo mkdir -p '%s'`, filepath.Dir(file.Destination))))
	if err != nil {
		return WrapErrorWith(err, stdout, stderr, "creating file's directory on target host")
	}

	stdout, stderr, err = ssh.Run(ctx, command.New(fmt.Sprintf(`sudo mv %s %s`, tmpPath, file.Destination)))
	if err != nil {
		return WrapErrorWith(err, stdout, stderr, "moving file to destination path")
	}

	return nil
}
