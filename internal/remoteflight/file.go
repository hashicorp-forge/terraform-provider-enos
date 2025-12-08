// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package remoteflight

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/random"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/retry"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
)

// CopyFileRequest copies a file to the remote host.
type CopyFileRequest struct {
	Content     it.Copyable
	TmpDir      string
	Chmod       string
	Chown       string
	Destination string
	RetryOpts   []retry.RetrierOpt
}

// DeleteFileRequest deletes a file on the remote host.
type DeleteFileRequest struct {
	Path      string
	RetryOpts []retry.RetrierOpt
}

// CopyFileRequestOpt is a functional option for file copy.
type CopyFileRequestOpt func(*CopyFileRequest) *CopyFileRequest

// DeleteFileRequestOpt is a functional option for file deletion.
type DeleteFileRequestOpt func(*DeleteFileRequest) *DeleteFileRequest

// NewCopyFileRequest takes functional options and returns a new file copy request.
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

// NewDeleteFileRequest takes functional options and returns a new file deletion request.
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

// WithCopyFileContent sets content to be copied.
func WithCopyFileContent(content it.Copyable) CopyFileRequestOpt {
	return func(cf *CopyFileRequest) *CopyFileRequest {
		cf.Content = content
		return cf
	}
}

// WithCopyFileTmpDir sets temporary directory to use.
func WithCopyFileTmpDir(dir string) CopyFileRequestOpt {
	return func(cf *CopyFileRequest) *CopyFileRequest {
		cf.TmpDir = dir
		return cf
	}
}

// WithCopyFileChmod sets permissions.
func WithCopyFileChmod(chmod string) CopyFileRequestOpt {
	return func(cf *CopyFileRequest) *CopyFileRequest {
		cf.Chmod = chmod
		return cf
	}
}

// WithCopyFileChown sets ownership.
func WithCopyFileChown(chown string) CopyFileRequestOpt {
	return func(cf *CopyFileRequest) *CopyFileRequest {
		cf.Chown = chown
		return cf
	}
}

// WithCopyFileDestination sets file destination.
func WithCopyFileDestination(destination string) CopyFileRequestOpt {
	return func(cf *CopyFileRequest) *CopyFileRequest {
		cf.Destination = destination
		return cf
	}
}

// WithCopyFileRetryOptions sets retry options for file copy operations.
func WithCopyFileRetryOptions(opts ...retry.RetrierOpt) CopyFileRequestOpt {
	return func(cf *CopyFileRequest) *CopyFileRequest {
		cf.RetryOpts = opts
		return cf
	}
}

// WithDeleteFileRetryOptions sets retry options for file delete operations.
func WithDeleteFileRetryOptions(opts ...retry.RetrierOpt) DeleteFileRequestOpt {
	return func(cf *DeleteFileRequest) *DeleteFileRequest {
		cf.RetryOpts = opts
		return cf
	}
}

// WithDeleteFilePath sets which file to delete for file delete operations.
func WithDeleteFilePath(path string) DeleteFileRequestOpt {
	return func(cf *DeleteFileRequest) *DeleteFileRequest {
		cf.Path = path
		return cf
	}
}

// CopyFile copies a file to the remote host. It first copies a file to a temporary
// directory, sets permissions, then copies to the destination directory as
// a superuser — retrying these operations if necessary.
func CopyFile(ctx context.Context, tr it.Transport, file *CopyFileRequest) error {
	if file == nil {
		return errors.New("no file copy request provided")
	}

	if file.Destination == "" {
		return errors.New("you must supply a destination path")
	}

	tmpPath := filepath.Join(file.TmpDir, fmt.Sprintf("%s-%s",
		filepath.Base(file.Destination),
		random.ID(),
	))

	fileOperations := func(ctx context.Context) (any, error) {
		var err error
		var stdout string
		var stderr string
		var res any

		err = tr.Copy(ctx, file.Content, tmpPath)
		if err != nil {
			return res, fmt.Errorf("copying file to target host: %w", err)
		}

		if file.Chmod != "" {
			stdout, stderr, err = tr.Run(ctx, command.New(fmt.Sprintf("chmod %s %s", file.Chmod, tmpPath)))
			if err != nil {
				err = WrapErrorWith(err, stdout, stderr, "changing file permissions")
				stdout, stderr, err1 := tr.Run(ctx, command.New(fmt.Sprintf("sudo chmod %s %s", file.Chmod, tmpPath)))
				if err1 != nil {
					err1 = WrapErrorWith(err1, stdout, stderr, "changing file permissions with sudo")
					return res, errors.Join(err, err1)
				}
			}
		}

		if file.Chown != "" {
			stdout, stderr, err = tr.Run(ctx, command.New(fmt.Sprintf("chown %s %s", file.Chown, tmpPath)))
			if err != nil {
				err = WrapErrorWith(err, stdout, stderr, "changing file ownership")
				stdout, stderr, err1 := tr.Run(ctx, command.New(fmt.Sprintf("sudo chown %s %s", file.Chown, tmpPath)))
				if err1 != nil {
					err1 = WrapErrorWith(err1, stdout, stderr, "changing file ownership with sudo")
					return res, errors.Join(err, err1)
				}
			}
		}

		stdout, stderr, err = tr.Run(ctx, command.New(fmt.Sprintf(`mkdir -p '%s'`, filepath.Dir(file.Destination))))
		if err != nil {
			err = WrapErrorWith(err, stdout, stderr, "creating file's directory on target host")
			stdout, stderr, err1 := tr.Run(ctx, command.New(fmt.Sprintf(`sudo mkdir -p '%s'`, filepath.Dir(file.Destination))))
			if err1 != nil {
				err1 = WrapErrorWith(err1, stdout, stderr, "creating file's directory on target host")
				return res, errors.Join(err, err1)
			}
		}

		cmd := fmt.Sprintf(`mv %s %s`, tmpPath, file.Destination)
		stdout, stderr, err = tr.Run(ctx, command.New(cmd))
		if err != nil {
			err = WrapErrorWith(err, stdout, stderr, "moving file to destination path, cmd: "+cmd)
			cmd = fmt.Sprintf(`sudo mv %s %s`, tmpPath, file.Destination)
			stdout, stderr, err1 := tr.Run(ctx, command.New(cmd))
			if err1 != nil {
				err1 = WrapErrorWith(
					err1, stdout, stderr,
					"moving file to destination path with sudo, cmd: "+cmd,
				)

				return res, errors.Join(err, err1)
			}
		}

		return res, nil
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

// DeleteFile deletes a file on the remote host, retrying if necessary.
func DeleteFile(ctx context.Context, tr it.Transport, req *DeleteFileRequest) error {
	if req == nil {
		return errors.New("no file delete request provided")
	}

	rmFile := func(ctx context.Context) (any, error) {
		var res any
		stdout, stderr, err := tr.Run(ctx, command.New("rm -r "+req.Path+" || sudo rm -r "+req.Path))
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
