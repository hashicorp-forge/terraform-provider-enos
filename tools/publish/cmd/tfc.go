package main

import (
	"github.com/spf13/cobra"
)

func newTFCCmd() *cobra.Command {
	tfcCmd := &cobra.Command{
		Use:   "tfc [COMMANDS]",
		Short: "tfc command to upload or download the artifacts for a private registry in TFC org",
	}

	tfcCmd.AddCommand(newTFCUploadCmd())
	// TODO: tfcCmd.AddCommand(newTFCDownloadCmd())

	return tfcCmd
}
