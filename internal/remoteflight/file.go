package remoteflight

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/hashicorp/enos-provider/internal/random"
	"github.com/hashicorp/enos-provider/internal/retry"
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
	RetryOpts   []retry.RetrierOpt
}

// DeleteFileRequest deletes a file on the remote host
type DeleteFileRequest struct {
	Path      string
	RetryOpts []retry.RetrierOpt
}

// CopyFileRequestOpt is a functional option for file copy
type CopyFileRequestOpt func(*CopyFileRequest) *CopyFileRequest

// DeleteFileRequestOpt is a functional option for file deletion
type DeleteFileRequestOpt func(*DeleteFileRequest) *DeleteFileRequest

// NewCopyFileRequest takes functional options and returns a new file copy request
func NewCopyFileRequest(opts ...CopyFileRequestOpt) *CopyFileRequest {
	cf := &CopyFileRequest{
		TmpDir: "/tmp",
		RetryOpts: []retry.RetrierOpt{
			retry.WithMaxRetries(3),
			retry.WithIntervalFunc(retry.IntervalFibonacci(time.Second)),
		},
	}

	for _, opt := range opts {
		cf = opt(cf)
	}

	return cf
}

// NewDeleteFileRequest takes functional options and returns a new file deletion request
func NewDeleteFileRequest(opts ...DeleteFileRequestOpt) *DeleteFileRequest {
	cf := &DeleteFileRequest{
		RetryOpts: []retry.RetrierOpt{
			retry.WithMaxRetries(3),
			retry.WithIntervalFunc(retry.IntervalFibonacci(time.Second)),
		},
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

// WithCopyFileRetryOptions sets retry options for file copy operations
func WithCopyFileRetryOptions(opts ...retry.RetrierOpt) CopyFileRequestOpt {
	return func(cf *CopyFileRequest) *CopyFileRequest {
		cf.RetryOpts = opts
		return cf
	}
}

// WithDeleteFileRetryOptions sets retry options for file delete operations
func WithDeleteFileRetryOptions(opts ...retry.RetrierOpt) DeleteFileRequestOpt {
	return func(cf *DeleteFileRequest) *DeleteFileRequest {
		cf.RetryOpts = opts
		return cf
	}
}

// WithDeleteFilePath sets which file to delete for file delete operations
func WithDeleteFilePath(path string) DeleteFileRequestOpt {
	return func(cf *DeleteFileRequest) *DeleteFileRequest {
		cf.Path = path
		return cf
	}
}

// CopyFile copies a file to the remote host. It first copies a file to a temporary
// directory, sets permissions, then copies to the destination directory as
// a superuser — retrying these operations if necessary.
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

	fileOperations := func(ctx context.Context) (interface{}, error) {
		var err error
		var stdout string
		var stderr string
		var res interface{}

		err = ssh.Copy(ctx, file.Content, tmpPath)
		if err != nil {
			return res, fmt.Errorf("copying file to target host: %w", err)
		}

		if file.Chmod != "" {
			stderr, stdout, err = ssh.Run(ctx, command.New(fmt.Sprintf("sudo chmod %s %s", file.Chmod, tmpPath)))
			if err != nil {
				return res, WrapErrorWith(err, stdout, stderr, "changing file permissions")
			}
		}

		if file.Chown != "" {
			stderr, stdout, err = ssh.Run(ctx, command.New(fmt.Sprintf("sudo chown %s %s", file.Chown, tmpPath)))
			if err != nil {
				return res, WrapErrorWith(err, stdout, stderr, "changing file ownership")
			}
		}

		stdout, stderr, err = ssh.Run(ctx, command.New(fmt.Sprintf(`sudo mkdir -p '%s'`, filepath.Dir(file.Destination))))
		if err != nil {
			return res, WrapErrorWith(err, stdout, stderr, "creating file's directory on target host")
		}

		stdout, stderr, err = ssh.Run(ctx, command.New(fmt.Sprintf(`sudo mv %s %s`, tmpPath, file.Destination)))
		if err != nil {
			return res, WrapErrorWith(err, stdout, stderr, "moving file to destination path")
		}

		return res, err
	}

	opts := append(file.RetryOpts, retry.WithRetrierFunc(fileOperations))
	r, err := retry.NewRetrier(opts...)
	if err != nil {
		return err
	}

	_, err = retry.Retry(ctx, r)
	if err != nil {
		return err
	}

	return nil
}

// DeleteFile deletes a file on the remote host, retrying if necessary
func DeleteFile(ctx context.Context, ssh it.Transport, req *DeleteFileRequest) error {
	if req == nil {
		return fmt.Errorf("no file delete request provided")
	}

	rmFile := func(ctx context.Context) (interface{}, error) {
		var res interface{}
		stderr, stdout, err := ssh.Run(ctx, command.New(fmt.Sprintf("sudo rm -r %s", req.Path)))
		if err != nil {
			return res, WrapErrorWith(err, stdout, stderr, "deleting file")
		}

		return res, err
	}

	opts := append(req.RetryOpts, retry.WithRetrierFunc(rmFile))
	r, err := retry.NewRetrier(opts...)
	if err != nil {
		return err
	}

	_, err = retry.Retry(ctx, r)
	if err != nil {
		return err
	}

	return nil
}
