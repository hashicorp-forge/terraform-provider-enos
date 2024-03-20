// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package download

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/cli"
)

// Command is a cli.Command.
type Command struct {
	ui   cli.Ui
	args *CommandArgs
}

// NewCommand takes a user interface and returns a new cli.Command.
func NewCommand(ui cli.Ui) (*Command, error) {
	return &Command{
		ui:   ui,
		args: &CommandArgs{},
	}, nil
}

// CommandArgs are the download commands arguments.
type CommandArgs struct {
	flags                     *flag.FlagSet
	destination               string
	writeStdout               bool
	url                       string
	timeout                   time.Duration
	mode                      int
	sha256                    string
	authUser                  string
	authPassword              string
	replace                   bool
	exitWithRequestStatusCode bool
}

// Synopsis is the cli.Command synopsis.
func (c *Command) Synopsis() string {
	return "Download a file"
}

// Help is the cli.Command help.
func (c *Command) Help() string {
	help := `
Usage: enos-flight-control download --url https://some/remote/file.txt --destination /local/path/file.txt --mode 0755 --timeout 5m --sha256 02b3...

  Downloads a file

Options:

  --url                     The URL of the file you wish to download
  --destination             The local destination where you wish to write the file
  --stdout                  Write the output to stdout
  --mode                    The desired file permissions of the downloaded file
  --timeout                 The maximum allowable request time, eg: 1m
  --sha256                  Verifies that the downloaded file matches the given SHA 256 sum
  --auth-user               The username to use for basic auth
  --auth-password           The password to use for basic auth
  --replace                 Replace the destination file if it exists
  --exit-with-status-code   On failure, exit with the HTTP status code returned

`

	return strings.TrimSpace(help)
}

// Run is the main cli.Command execution function.
func (c *Command) Run(args []string) int {
	err := c.args.Parse(args)
	if err != nil {
		c.ui.Error(err.Error())
		return 1
	}

	err = c.Download()
	if err == nil {
		return 0
	}

	exitCode := 1
	if c.args.exitWithRequestStatusCode {
		var errDownload *ErrDownload
		if errors.As(err, &errDownload) {
			exitCode = errDownload.StatusCode
		}
	}

	c.ui.Error(err.Error())

	return exitCode
}

// Parse parses the raw args and maps them to the CommandArgs.
func (a *CommandArgs) Parse(args []string) error {
	a.flags = flag.NewFlagSet("download", flag.ContinueOnError)
	a.flags.StringVar(&a.destination, "destination", "", "where to write the resulting file")
	a.flags.BoolVar(&a.writeStdout, "stdout", false, "Write the output to stdout")
	a.flags.StringVar(&a.url, "url", "", "the URL of the resource you wish to download")
	a.flags.IntVar(&a.mode, "mode", 0o666, "the file permissions of the downloaded file")
	a.flags.DurationVar(&a.timeout, "timeout", 5*time.Minute, "the maximum allowable request time")
	a.flags.StringVar(&a.sha256, "sha256", "", "if given, verifies that the downloaded file matches the given SHA 256 sum")
	a.flags.StringVar(&a.authUser, "auth-user", "", "if given, sets the basic auth username when making the HTTP request")
	a.flags.StringVar(&a.authPassword, "auth-password", "", "if given, sets the basic auth password when making the HTTP request")
	a.flags.BoolVar(&a.replace, "replace", false, "overwite the destination if it already exists")
	a.flags.BoolVar(&a.exitWithRequestStatusCode, "exit-with-status-code", false, "On failure, exit with the HTTP status code returned")

	err := a.flags.Parse(args)
	if err != nil {
		return err
	}

	if !a.writeStdout && a.destination == "" {
		return errors.New("you must provide either a destination or stdout")
	}

	return nil
}

// Download downloads the file and writes it to the destination.
func (c *Command) Download() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.args.timeout)
	defer cancel()

	opts := []RequestOpt{
		WithRequestURL(c.args.url),
		WithRequestSHA256(c.args.sha256),
		WithRequestWriteStdout(c.args.writeStdout),
	}

	if c.args.destination != "" {
		_, err := os.Stat(c.args.destination)
		if err == nil {
			// The destination file already exists
			if !c.args.replace {
				return fmt.Errorf("%s already exists. Set --replace=true to replace existing files", c.args.destination)
			}

			err = os.Remove(c.args.destination)
			if err != nil {
				return err
			}
		}

		dst, err := os.OpenFile(c.args.destination, os.O_RDWR|os.O_CREATE, fs.FileMode(c.args.mode))
		if err != nil {
			return err
		}
		defer dst.Close()

		opts = append(opts, WithRequestDestination(dst))
	}

	if c.args.authUser != "" {
		opts = append(opts, WithRequestAuthUser(c.args.authUser))
	}

	if c.args.authPassword != "" {
		opts = append(opts, WithRequestAuthPassword(c.args.authPassword))
	}

	req, err := NewRequest(opts...)
	if err != nil {
		return err
	}

	return Download(ctx, req)
}
