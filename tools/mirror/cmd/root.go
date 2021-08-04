package main

import (
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type rootConfig struct {
	requestTimeout time.Duration
}

var rootCfg = &rootConfig{}

// Level sets the log level
var Level = zap.LevelFlag("log", zap.InfoLevel, "the log level (error, warn, info, debug, trace)") // declare global

func newRootCommand() *cobra.Command {
	// rootCmd represents the base command when called without any subcommands
	rootCmd := &cobra.Command{
		Use:              "mirror [COMMANDS]",
		TraverseChildren: true,
		Short:            "A tool to create and promote a remote S3 mirror used to install private Terraform provider",
		Long:             `mirror is a CLI tool intended to take the output of goreleaser and create a remote mirror in S3 or promote created artifact to another S3 mirror, that Terraform can use to install the provider. This allows us to distribute the provider without running an internal private provider registry.`,
	}

	rootCmd.PersistentFlags().DurationVar(&rootCfg.requestTimeout, "timeout", time.Duration(15*time.Minute), "maximum allowed time to run")

	return rootCmd
}
