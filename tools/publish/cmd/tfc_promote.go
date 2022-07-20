package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/hashicorp/enos-provider/tools/publish/pkg/publish"
)

var tfcPromoteCfg = &publish.TFCPromoteReq{}

func newTFCPromoteCmd() *cobra.Command {
	tfcPromoteCmd := &cobra.Command{
		Use:   "promote [ARGS]",
		Short: "Promote the generated artifacts to private registry in TFC org",
		Run:   runTFCPromoteCmd,
	}

	tfcPromoteCmd.PersistentFlags().StringVar(&tfcPromoteCfg.ProviderVersion, "provider-version", "", "the version of the provider binaries")
	tfcPromoteCmd.PersistentFlags().StringVar(&tfcPromoteCfg.DownloadsDir, "downloads-dir", "enos-downloads", "the source directory containing the build the artifacts to be promoted")
	tfcPromoteCmd.PersistentFlags().StringVar(&tfcPromoteCfg.PromoteDir, "promote-dir", "enos-promote", "the output directory where the downloaded build artifacts are extracted and promoted")
	tfcPromoteCmd.PersistentFlags().StringVar(&tfcPromoteCfg.SrcProviderName, "src-provider-name", "enosdev", "the name of the source TFC provider")
	tfcPromoteCmd.PersistentFlags().StringVar(&tfcPromoteCfg.DestProviderName, "dest-provider-name", "enos", "the name of the destination TFC provider")
	tfcPromoteCmd.PersistentFlags().StringVar(&tfcPromoteCfg.SrcBinaryName, "src-binary-name", "terraform-provider-enosdev", "the name of the source provider binary")
	tfcPromoteCmd.PersistentFlags().StringVar(&tfcPromoteCfg.DestBinaryName, "dest-binary-name", "terraform-provider-enos", "the name of the destination provider binary")
	tfcPromoteCmd.PersistentFlags().StringVar(&tfcPromoteCfg.GPGKeyID, "gpg-key-id", "5D67D7B072C16294", "the GPG Signing Key")
	tfcPromoteCmd.PersistentFlags().StringVar(&tfcPromoteCfg.GPGIdentityName, "gpg-identity-name", "team-secure-quality@hashicorp.com", "the GPG identity name, should be an email address")
	tfcPromoteCmd.PersistentFlags().StringVar(&tfcPromoteCfg.TFCOrg, "org", "hashicorp-qti", "the name of the TFC org")
	tfcPromoteCmd.PersistentFlags().StringVar(&tfcPromoteCfg.TFCToken, "token", "", "the TFC token for the org")

	_ = tfcPromoteCmd.MarkFlagRequired("provider-version")
	_ = tfcPromoteCmd.MarkFlagRequired("downloads-dir")
	_ = tfcPromoteCmd.MarkFlagRequired("promote-dir")
	_ = tfcPromoteCmd.MarkFlagRequired("src-provider-name")
	_ = tfcPromoteCmd.MarkFlagRequired("dest-provider-name")
	_ = tfcPromoteCmd.MarkFlagRequired("src-binary-name")
	_ = tfcPromoteCmd.MarkFlagRequired("dest-binary-name")
	_ = tfcPromoteCmd.MarkFlagRequired("gpg-key-id")
	_ = tfcPromoteCmd.MarkFlagRequired("gpg-identity-name")
	_ = tfcPromoteCmd.MarkFlagRequired("org")
	_ = tfcPromoteCmd.MarkFlagRequired("token")
	return tfcPromoteCmd
}

func runTFCPromoteCmd(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.requestTimeout)
	defer cancel()

	publishDownload := publish.NewLocal(tfcPromoteCfg.SrcProviderName, tfcPromoteCfg.SrcBinaryName)
	err := publishDownload.Initialize()
	exitIfErr(err)
	defer publishDownload.Close()

	exitIfErr(publishDownload.SetLogLevel(*Level))

	downloadCfg := &publish.TFCDownloadReq{
		DownloadDir:     tfcPromoteCfg.DownloadsDir,
		ProviderVersion: tfcPromoteCfg.ProviderVersion,
		BinaryName:      tfcPromoteCfg.SrcBinaryName,
		ProviderName:    tfcPromoteCfg.SrcProviderName,
		TFCOrg:          tfcPromoteCfg.TFCOrg,
		TFCToken:        tfcPromoteCfg.TFCToken,
	}
	exitIfErr(publishDownload.DownloadFromTFC(ctx, downloadCfg))

	publishPromote := publish.NewLocal(tfcPromoteCfg.DestProviderName, tfcPromoteCfg.SrcBinaryName, publish.WithLocalBinaryRename(tfcPromoteCfg.DestBinaryName))
	err = publishPromote.Initialize()
	exitIfErr(err)
	defer publishPromote.Close()

	exitIfErr(publishPromote.SetLogLevel(*Level))

	exitIfErr(publishPromote.ExtractProviderBinaries(ctx, tfcPromoteCfg))

	exitIfErr(publishPromote.AddGoreleaserBinariesFrom(tfcPromoteCfg.PromoteDir))

	exitIfErr(publishPromote.WriteSHA256Sums(ctx, tfcPromoteCfg.GPGIdentityName, true))

	uploadCfg := &publish.TFCUploadReq{
		DistDir:         tfcPromoteCfg.PromoteDir,
		BinaryRename:    tfcPromoteCfg.DestBinaryName,
		BinaryName:      tfcPromoteCfg.SrcBinaryName,
		ProviderName:    tfcPromoteCfg.DestProviderName,
		GPGKeyID:        tfcPromoteCfg.GPGKeyID,
		GPGIdentityName: tfcPromoteCfg.GPGIdentityName,
		TFCOrg:          tfcPromoteCfg.TFCOrg,
		TFCToken:        tfcPromoteCfg.TFCToken,
	}
	exitIfErr(publishPromote.PublishToTFC(ctx, uploadCfg))
}
