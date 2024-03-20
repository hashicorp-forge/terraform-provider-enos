// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package remoteflight

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/random"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/retry"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/ui"
)

// RunScriptRequest copies a file to the remote host.
type RunScriptRequest struct {
	Env             map[string]string
	NoCleanup       bool
	Sudo            bool
	CopyFileRequest *CopyFileRequest
}

// RunScriptResponse is the response of the script run.
type RunScriptResponse struct {
	Stdout string
	Stderr string
}

// RunScriptRequestOpt is a functional option for running a script.
type RunScriptRequestOpt func(*RunScriptRequest) *RunScriptRequest

// NewRunScriptRequest takes functional options and returns a new script run req.
func NewRunScriptRequest(opts ...RunScriptRequestOpt) *RunScriptRequest {
	cf := &RunScriptRequest{
		Env:       map[string]string{},
		NoCleanup: false,
		Sudo:      false,
		CopyFileRequest: &CopyFileRequest{
			Destination: "/tmp/remoteflight_run_script_" + random.ID(),
			TmpDir:      "/tmp",
			// without this, the script will be run infinite times and block forever if it fails
			RetryOpts: []retry.RetrierOpt{
				retry.WithMaxRetries(3),
				retry.WithIntervalFunc(retry.IntervalFibonacci(time.Second)),
			},
		},
	}

	for _, opt := range opts {
		cf = opt(cf)
	}

	return cf
}

// WithRunScriptEnv sets the environment variables.
func WithRunScriptEnv(env map[string]string) RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		for k, v := range env {
			cf.Env[k] = v
		}

		return cf
	}
}

// WithRunScriptNoCleanup disable the auto cleanup.
func WithRunScriptNoCleanup() RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.NoCleanup = true
		return cf
	}
}

// WithRunScriptUseSudo runs the script with sudo.
func WithRunScriptUseSudo() RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.Sudo = true
		return cf
	}
}

// WithRunScriptContent sets content to be copied.
func WithRunScriptContent(content it.Copyable) RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.CopyFileRequest.Content = content
		return cf
	}
}

// WithRunScriptTmpDir sets temporary directory to use.
func WithRunScriptTmpDir(dir string) RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.CopyFileRequest.TmpDir = dir
		return cf
	}
}

// WithRunScriptChmod sets permissions.
func WithRunScriptChmod(chmod string) RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.CopyFileRequest.Chmod = chmod
		return cf
	}
}

// WithRunScriptChown sets ownership.
func WithRunScriptChown(chown string) RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.CopyFileRequest.Chown = chown
		return cf
	}
}

// WithRunScriptDestination sets file destination.
func WithRunScriptDestination(destination string) RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.CopyFileRequest.Destination = destination
		return cf
	}
}

// RunScript copies the script to the remote host, executes it, and cleans it up.
func RunScript(ctx context.Context, tr it.Transport, req *RunScriptRequest) (*RunScriptResponse, error) {
	var err error
	res := &RunScriptResponse{}
	ui := ui.NewBuffered()

	err = CopyFile(ctx, tr, req.CopyFileRequest)
	if err != nil {
		return res, err
	}

	cmd := req.CopyFileRequest.Destination
	if req.Sudo {
		cmd = "sudo " + cmd
	}
	stdout, stderr, err1 := tr.Run(ctx, command.New(cmd, command.WithEnvVars(req.Env)))
	err = errors.Join(err, err1)
	err = errors.Join(err, ui.Append(stdout, stderr))
	res.Stderr = ui.StderrString()
	res.Stdout = ui.StdoutString()
	if err != nil {
		return res, fmt.Errorf("executing script: %w: %s", err, stderr)
	}

	if req.NoCleanup {
		return res, nil
	}

	stdout, stderr, err1 = tr.Run(
		ctx, command.New("rm -f "+req.CopyFileRequest.Destination, command.WithEnvVars(req.Env)),
	)
	err = errors.Join(err, err1)
	err = errors.Join(err, ui.Append(stdout, stderr))
	res.Stderr = ui.StderrString()
	res.Stdout = ui.StdoutString()
	if err != nil {
		return res, fmt.Errorf("cleaning up script file: %w", err)
	}

	return res, nil
}
