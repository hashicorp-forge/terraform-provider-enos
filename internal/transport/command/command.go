// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"fmt"
	"strings"

	it "github.com/hashicorp/enos-provider/internal/transport"
)

type cmd struct {
	env map[string]string
	cmd string
}

var _ it.Command = (*cmd)(nil)

// Opt is a functional option.
type Opt func(*cmd)

// New takes zero or more functional options and return a new command.
func New(command string, opts ...Opt) it.Command {
	c := &cmd{
		cmd: command,
		env: map[string]string{},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WithEnvVars sets the environment variables.
func WithEnvVars(vars map[string]string) func(*cmd) {
	return func(c *cmd) {
		for key, val := range vars {
			c.env[key] = val
		}
	}
}

// WithEnvVar sets the environment variable.
func WithEnvVar(key, value string) func(*cmd) {
	return func(c *cmd) {
		c.env[key] = value
	}
}

func (c *cmd) Cmd() string {
	cmd := strings.Builder{}

	for key, val := range c.env {
		cmd.WriteString(fmt.Sprintf("%s='%s' ", key, val))
	}

	cmd.WriteString(c.cmd)

	return cmd.String()
}
