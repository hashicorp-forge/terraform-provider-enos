package main

import (
	"github.com/spf13/cobra"
)

func newS3Cmd() *cobra.Command {
	s3Cmd := &cobra.Command{
		Use:       "s3 [COMMANDS]",
		Short:     "s3 command to upload or copy the artifacts with given version to a remote S3 mirror",
		Args:      cobra.OnlyValidArgs,
		ValidArgs: []string{"upload", "copy"},
	}

	s3Cmd.AddCommand(news3UploadCmd())
	s3Cmd.AddCommand(news3CopyCmd())

	return s3Cmd
}
