// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package zip

import (
	"archive/zip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/cli"
)

// UnzipCommand is the unzip command.
type UnzipCommand struct {
	ui   cli.Ui
	args *UnzipCommandArgs
}

// NewUnzipCommand takes a user interface and returns a new cli.Command.
func NewUnzipCommand(ui cli.Ui) (*UnzipCommand, error) {
	return &UnzipCommand{
		ui:   ui,
		args: &UnzipCommandArgs{},
	}, nil
}

// UnzipCommandArgs are the unzip command's arguments.
type UnzipCommandArgs struct {
	flags           *flag.FlagSet
	source          string
	destination     string
	createDest      bool
	destinationMode int
	mode            int
	replace         bool
}

// Synopsis is the cli.Command synopsis.
func (c *UnzipCommand) Synopsis() string {
	return "Unzip an archive"
}

// Help is the cli.Command help.
func (c *UnzipCommand) Help() string {
	help := `
Usage: enos-flight-control unzip --source /some/file.zip --destination /some/directory --create-destination true

  Unzips a zip archive

Options:

  --source               The source zip archive
  --destination          The destination directory
  --create-destination   Create destination directory
  --destination-mode     The destination directory mode if creating it
  --mode                 The desired file permissions of the expanded archive files
  --replace              Replace any existing files with matching path

`

	return strings.TrimSpace(help)
}

// Run is the cli.Command main execution function.
func (c *UnzipCommand) Run(args []string) int {
	err := c.args.Parse(args)
	if err != nil {
		c.ui.Error(err.Error())

		return 1
	}

	err = c.Unzip()
	if err != nil {
		c.ui.Error(err.Error())

		return 1
	}

	return 0
}

// Parse parses the arguments and maps them to the UnzipCommandArgs.
func (a *UnzipCommandArgs) Parse(args []string) error {
	a.flags = flag.NewFlagSet("unzip", flag.ContinueOnError)
	a.flags.StringVar(&a.source, "source", "", "the source destination directory")
	a.flags.IntVar(&a.mode, "mode", 0o666, "the file permissions of the expanded archive files")
	a.flags.StringVar(&a.destination, "destination", "", "where to write the resulting file")
	a.flags.BoolVar(&a.createDest, "create-destination", true, "create the destination directory if necessary")
	a.flags.IntVar(&a.destinationMode, "destination-mode", 0o755, "the file permissions of the expanded archive files")
	a.flags.BoolVar(&a.replace, "replace", false, "replace any existing files with matching paths")

	err := a.flags.Parse(args)
	if err != nil {
		return err
	}

	return nil
}

// Unzip performs the unzip request.
func (c *UnzipCommand) Unzip() error {
	// Make sure we've got a zip file
	archive, err := zip.OpenReader(c.args.source)
	if err != nil {
		return err
	}
	defer archive.Close()

	fileMode := fs.FileMode(c.args.mode) //#nosec:G115

	// Make sure we've got a destination directory
	dstDir, err := os.Open(c.args.destination)
	if err != nil {
		if !c.args.createDest {
			return err
		}

		_, ok := err.(*fs.PathError)
		if !ok {
			return err
		}

		err = os.MkdirAll(c.args.destination, fs.FileMode(c.args.destinationMode)) //#nosec:G115
		if err != nil {
			return err
		}
	} else {
		defer dstDir.Close()

		s, err := dstDir.Stat()
		if err != nil {
			return err
		}

		if !s.IsDir() {
			return errors.New("destination path exists but is not a directory")
		}
	}

	// Expand the files into the destination directory
	for _, file := range archive.File {
		dstPath := filepath.Join(c.args.destination, file.Name)

		_, err := os.Stat(dstPath)
		if err == nil {
			// The destination file already exists
			if !c.args.replace {
				return fmt.Errorf("%s already exists. Set --replace=true to replace existing files", dstPath)
			}

			err = os.RemoveAll(dstPath)
			if err != nil {
				return err
			}
		}

		dst, err := os.OpenFile(dstPath, os.O_RDWR|os.O_CREATE, fileMode)
		if err != nil {
			return err
		}
		defer dst.Close()

		src, err := file.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		_, err = io.Copy(dst, src)
		if err != nil {
			return err
		}
	}

	return nil
}
