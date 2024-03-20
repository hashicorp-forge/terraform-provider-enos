// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	tfcCmd.AddCommand(newTFCDownloadCmd())
	tfcCmd.AddCommand(newTFCPromoteCmd())

	return tfcCmd
}
