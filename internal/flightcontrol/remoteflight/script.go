package remoteflight

import (
	"context"
	"fmt"

	"github.com/hashicorp/enos-provider/internal/random"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
	"github.com/hashicorp/enos-provider/internal/ui"
	"github.com/hashicorp/go-multierror"
)

// RunScriptRequest copies a file to the remote host
type RunScriptRequest struct {
	Env             map[string]string
	NoCleanup       bool
	Sudo            bool
	CopyFileRequest *CopyFileRequest
}

// RunScriptResponse is the response of the script run
type RunScriptResponse struct {
	Stdout string
	Stderr string
}

// RunScriptRequestOpt is a functional option for running a script
type RunScriptRequestOpt func(*RunScriptRequest) *RunScriptRequest

// NewRunScriptRequest takes functional options and returns a new script run req
func NewRunScriptRequest(opts ...RunScriptRequestOpt) *RunScriptRequest {
	cf := &RunScriptRequest{
		Env:       map[string]string{},
		NoCleanup: false,
		Sudo:      false,
		CopyFileRequest: &CopyFileRequest{
			Destination: fmt.Sprintf("/tmp/remoteflight_run_script_%s", random.ID()),
			TmpDir:      "/tmp",
		},
	}

	for _, opt := range opts {
		cf = opt(cf)
	}

	return cf
}

// WithRunScriptEnv sets the environment variables
func WithRunScriptEnv(env map[string]string) RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		for k, v := range env {
			cf.Env[k] = v
		}
		return cf
	}
}

// WithRunScriptNoCleanup disable the auto cleanup
func WithRunScriptNoCleanup() RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.NoCleanup = true
		return cf
	}
}

// WithRunScriptUseSudo runs the script with sudo
func WithRunScriptUseSudo() RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.Sudo = true
		return cf
	}
}

// WithRunScriptContent sets content to be copied
func WithRunScriptContent(content it.Copyable) RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.CopyFileRequest.Content = content
		return cf
	}
}

// WithRunScriptTmpDir sets temporary directory to use
func WithRunScriptTmpDir(dir string) RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.CopyFileRequest.TmpDir = dir
		return cf
	}
}

// WithRunScriptChmod sets permissions
func WithRunScriptChmod(chmod string) RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.CopyFileRequest.Chmod = chmod
		return cf
	}
}

// WithRunScriptChown sets ownership
func WithRunScriptChown(chown string) RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.CopyFileRequest.Chown = chown
		return cf
	}
}

// WithRunScriptDestination sets file destination
func WithRunScriptDestination(destination string) RunScriptRequestOpt {
	return func(cf *RunScriptRequest) *RunScriptRequest {
		cf.CopyFileRequest.Destination = destination
		return cf
	}
}

// RunScript copies the script to the remote host, executes it, and cleans it up.
func RunScript(ctx context.Context, ssh it.Transport, req *RunScriptRequest) (*RunScriptResponse, error) {
	merr := &multierror.Error{}
	res := &RunScriptResponse{}
	ui := ui.NewBuffered()

	err := CopyFile(ctx, ssh, req.CopyFileRequest)
	if err != nil {
		return res, err
	}

	cmd := req.CopyFileRequest.Destination
	if req.Sudo {
		cmd = fmt.Sprintf("sudo %s", cmd)
	}
	stdout, stderr, err := ssh.Run(ctx, command.New(cmd, command.WithEnvVars(req.Env)))
	merr = multierror.Append(merr, err)
	merr = multierror.Append(merr, ui.Append(stdout, stderr))
	res.Stderr = ui.Stderr().String()
	res.Stdout = ui.Stdout().String()
	if merr.ErrorOrNil() != nil {
		return res, fmt.Errorf("executing script: %w", merr)
	}

	if req.NoCleanup {
		return res, nil
	}

	stdout, stderr, err = ssh.Run(
		ctx, command.New(fmt.Sprintf("rm -f %s", req.CopyFileRequest.Destination), command.WithEnvVars(req.Env)),
	)
	merr = multierror.Append(merr, err)
	merr = multierror.Append(merr, ui.Append(stdout, stderr))
	res.Stderr = ui.Stderr().String()
	res.Stdout = ui.Stdout().String()
	if merr.ErrorOrNil() != nil {
		return res, fmt.Errorf("cleaning up script file: %w", merr)
	}

	return res, nil
}
