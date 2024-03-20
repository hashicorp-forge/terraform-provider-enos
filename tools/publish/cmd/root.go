// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := newRootCommand()
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type rootConfig struct {
	requestTimeout time.Duration
	logLevel       string
}

var rootCfg = &rootConfig{}

func newRootCommand() *cobra.Command {
	// rootCmd represents the base command when called without any subcommands
	rootCmd := &cobra.Command{
		Use:              "publish [COMMANDS]",
		TraverseChildren: true,
		Short:            "A tool to upload Terraform provider binaries to an S3 mirror, a private TFC org, or the public registry",
		Long:             "publish is a CLI tool intended to take the output of this provider build and upload it to a remote mirror in S3, to a private TFC provider registry, or the public registry",
	}

	rootCmd.PersistentFlags().DurationVar(&rootCfg.requestTimeout, "timeout", 15*time.Minute, "maximum allowed time to run")
	rootCmd.PersistentFlags().StringVar(&rootCfg.logLevel, "log-level", "info", "the log level (error, warn, info, debug, trace)")

	rootCmd.AddCommand(newTFCCmd())
	rootCmd.AddCommand(newGithubCmd())

	return rootCmd
}

func exitIfErr(err error) {
	if err != nil {
		fmt.Printf("ERROR: %s", err.Error())
		os.Exit(1)
	}
}
