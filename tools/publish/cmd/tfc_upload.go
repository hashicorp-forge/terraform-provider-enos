package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/hashicorp/enos-provider/tools/publish/pkg/publish"
)

var tfcUploadCfg = &publish.TFCUploadReq{}

func newTFCUploadCmd() *cobra.Command {
	tfcUploadCmd := &cobra.Command{
		Use:   "upload [ARGS]",
		Short: "Upload the generated artifacts to private registry in TFC org",
		Run:   runTFCUploadCmd,
	}

	tfcUploadCmd.PersistentFlags().StringVar(&tfcUploadCfg.DistDir, "dist", "", "the output directory of goreleaser that build the artifacts")
	tfcUploadCmd.PersistentFlags().StringVar(&tfcUploadCfg.ProviderName, "provider-name", "enos", "the name of the provider")
	tfcUploadCmd.PersistentFlags().StringVar(&tfcUploadCfg.BinaryName, "binary-name", "terraform-provider-enos", "the name of the provider binary")
	tfcUploadCmd.PersistentFlags().StringVar(&tfcUploadCfg.BinaryRename, "rename-binary", "", "the desired provider binary name during upload")
	tfcUploadCmd.PersistentFlags().StringVar(&tfcUploadCfg.GPGKeyID, "gpg-key-id", "5D67D7B072C16294", "the GPG Signing Key")
	tfcUploadCmd.PersistentFlags().StringVar(&tfcUploadCfg.GPGIdentityName, "gpg-identity-name", "team-secure-quality@hashicorp.com", "the GPG identity name, should be an email address")
	tfcUploadCmd.PersistentFlags().StringVar(&tfcUploadCfg.TFCOrg, "org", "hashicorp-qti", "the name of the TFC org")
	tfcUploadCmd.PersistentFlags().StringVar(&tfcUploadCfg.TFCToken, "token", "", "the TFC token with publish permissions for the org")

	_ = tfcUploadCmd.MarkFlagRequired("dist")
	_ = tfcUploadCmd.MarkFlagRequired("provider-name")
	_ = tfcUploadCmd.MarkFlagRequired("gpg-key-id")
	_ = tfcUploadCmd.MarkFlagRequired("gpg-identity-name")
	_ = tfcUploadCmd.MarkFlagRequired("org")
	_ = tfcUploadCmd.MarkFlagRequired("token")
	return tfcUploadCmd
}

func runTFCUploadCmd(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.requestTimeout)
	defer cancel()

	publish := publish.NewLocal(tfcUploadCfg.ProviderName, tfcUploadCfg.BinaryName, publish.WithLocalBinaryRename(tfcUploadCfg.BinaryRename))
	err := publish.Initialize()
	exitIfErr(err)
	defer publish.Close()

	exitIfErr(publish.SetLogLevel(*Level))

	exitIfErr(publish.AddGoreleaserBinariesFrom(tfcUploadCfg.DistDir))

	exitIfErr(publish.WriteSHA256Sums(ctx, tfcUploadCfg.GPGIdentityName, true))

	exitIfErr(publish.PublishToTFC(ctx, tfcUploadCfg))

	os.Exit(0)
}
