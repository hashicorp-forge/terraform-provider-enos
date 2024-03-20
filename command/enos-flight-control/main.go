// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"os"

	"github.com/mitchellh/cli"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/flightcontrol/download"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/flightcontrol/zip"
)

var ui = &cli.BasicUi{
	Writer:      os.Stdout,
	ErrorWriter: os.Stderr,
	Reader:      os.Stdin,
}

func main() {
	runner := &cli.CLI{
		Name: os.Args[0],
		Args: os.Args[1:],
		Commands: map[string]cli.CommandFactory{
			"download": func() (cli.Command, error) {
				return download.NewCommand(ui)
			},
			"unzip": func() (cli.Command, error) {
				return zip.NewUnzipCommand(ui)
			},
		},
	}

	code, err := runner.Run()
	if err != nil {
		ui.Error(err.Error())
	}

	os.Exit(code)
}
