package download

import (
	"context"
	"flag"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/cli"
)

// Command is a cli.Command
type Command struct {
	ui   cli.Ui
	args *CommandArgs
}

// NewCommand takes a user interface and returns a new cli.Command
func NewCommand(ui cli.Ui) (*Command, error) {
	return &Command{
		ui:   ui,
		args: &CommandArgs{},
	}, nil
}

// CommandArgs are the download commands arguments
type CommandArgs struct {
	flags        *flag.FlagSet
	destination  string
	url          string
	timeout      time.Duration
	mode         int
	sha256       string
	authUser     string
	authPassword string
}

// Synopsis is the cli.Command synopsis
func (c *Command) Synopsis() string {
	return "Download a file"
}

// Help is the cli.Command help
func (c *Command) Help() string {
	help := `
Usage: enos-flight-control download --url https://some/remote/file.txt --destination /local/path/file.txt --mode 0755 --timeout 5m --sha256 02b3...

  Downloads a file

Options:

  --url            The URL of the file you wish to download
  --destination    The local destination where you wish to write the file
  --mode           The desired file permissions of the downloaded file
  --timeout        The maximum allowable request time, eg: 1m
  --sha256         Verifies that the downloaded file matches the given SHA 256 sum
  --auth-user      The username to use for basic auth
  --auth-password  The password to use for basic auth

`
	return strings.TrimSpace(help)
}

// Run is the main cli.Command execution function
func (c *Command) Run(args []string) int {
	err := c.args.Parse(args)
	if err != nil {
		c.ui.Error(err.Error())
		return 1
	}

	err = c.Download()
	if err != nil {
		c.ui.Error(err.Error())
		return 1
	}

	return 0
}

// Parse parses the raw args and maps them to the CommandArgs
func (a *CommandArgs) Parse(args []string) error {
	a.flags = flag.NewFlagSet("download", flag.ContinueOnError)
	a.flags.StringVar(&a.destination, "destination", "", "where to write the resulting file")
	a.flags.StringVar(&a.url, "url", "", "the URL of the resource you wish to download")
	a.flags.IntVar(&a.mode, "mode", 0o666, "the file permissions of the downloaded file")
	a.flags.DurationVar(&a.timeout, "timeout", 5*time.Minute, "the maximum allowable request time")
	a.flags.StringVar(&a.sha256, "sha256", "", "if given, verifies that the downloaded file matches the given SHA 256 sum")
	a.flags.StringVar(&a.authUser, "auth-user", "", "if given, sets the basic auth username when making the HTTP request")
	a.flags.StringVar(&a.authPassword, "auth-password", "", "if given, sets the basic auth password when making the HTTP request")

	err := a.flags.Parse(args)
	if err != nil {
		return err
	}

	return nil
}

// Download downloads the file and writes it to the destination
func (c *Command) Download() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.args.timeout)
	defer cancel()

	dst, err := os.OpenFile(c.args.destination, os.O_RDWR|os.O_CREATE, fs.FileMode(c.args.mode))
	if err != nil {
		return err
	}
	defer dst.Close()

	opts := []RequestOpt{
		WithRequestURL(c.args.url),
		WithRequestDestination(dst),
		WithRequestSHA256(c.args.sha256),
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
