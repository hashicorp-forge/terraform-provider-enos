package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"

	"github.com/hashicorp/enos-provider/tools/publish/pkg/publish"
)

var tfcDownloadCfg = &publish.TFCDownloadReq{}

func newTFCDownloadCmd() *cobra.Command {
	tfcDownloadCmd := &cobra.Command{
		Use:   "download [ARGS]",
		Short: "Download artifacts from private registry in TFC org",
		Run:   runTFCDownloadCmd,
	}

	tfcDownloadCmd.PersistentFlags().StringVar(&tfcDownloadCfg.DownloadDir, "download-dir", "enos-downloads", "the directory where the artifacts are downloaded")
	tfcDownloadCmd.PersistentFlags().StringVar(&tfcDownloadCfg.ProviderName, "provider-name", "enosdev", "the name of the provider")
	tfcDownloadCmd.PersistentFlags().StringVar(&tfcDownloadCfg.ProviderVersion, "provider-version", "", "the version of the provider binaries")
	tfcDownloadCmd.PersistentFlags().StringVar(&tfcDownloadCfg.BinaryName, "binary-name", "terraform-provider-enosdev", "the name of the provider binary")
	tfcDownloadCmd.PersistentFlags().StringVar(&tfcDownloadCfg.TFCOrg, "org", "hashicorp-qti", "the name of the TFC org")
	tfcDownloadCmd.PersistentFlags().StringVar(&tfcDownloadCfg.TFCToken, "token", "", "the TFC token with publish permissions for the org")

	_ = tfcDownloadCmd.MarkFlagRequired("download-dir")
	_ = tfcDownloadCmd.MarkFlagRequired("provider-name")
	_ = tfcDownloadCmd.MarkFlagRequired("provider-version")
	_ = tfcDownloadCmd.MarkFlagRequired("binary-name")
	_ = tfcDownloadCmd.MarkFlagRequired("org")
	_ = tfcDownloadCmd.MarkFlagRequired("token")

	return tfcDownloadCmd
}

func runTFCDownloadCmd(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.requestTimeout)
	defer cancel()

	publish := publish.NewLocal(tfcDownloadCfg.ProviderName, tfcDownloadCfg.BinaryName)
	err := publish.Initialize()
	exitIfErr(err)
	defer publish.Close()

	lvl, err := zapcore.ParseLevel(rootCfg.logLevel)
	exitIfErr(err)

	exitIfErr(publish.SetLogLevel(lvl))

	exitIfErr(publish.DownloadFromTFC(ctx, tfcDownloadCfg))

	os.Exit(0)
}
