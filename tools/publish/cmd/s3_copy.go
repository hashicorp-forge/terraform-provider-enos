package main

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"

	"github.com/hashicorp/enos-provider/tools/publish/pkg/publish"
)

var copyCfg = &publish.CopyReq{}

func news3CopyCmd() *cobra.Command {
	// copyCmd represents the copy command
	copyCmd := &cobra.Command{
		Use:   "copy --version [ARTIFACT_VERSION] --src-bucket [S3 BUCKETNAME] --dest-bucket [S3 BUCKETNAME]",
		Short: "copy the artifacts with given version from remote source S3 mirror to destination S3 mirror",
		Run:   runCopyCmd,
	}

	copyCmd.Flags().StringVar(&copyCfg.Version, "version", "", "the version of the artifact to be copyed")
	copyCmd.Flags().StringVar(&copyCfg.SrcBucketName, "src-bucket", "", "the Source S3 bucket name")
	copyCmd.Flags().StringVar(&copyCfg.DestBucketName, "dest-bucket", "", "the Destination S3 bucket name")
	copyCmd.PersistentFlags().StringVar(&copyCfg.SrcProviderName, "src-provider-name", "terraform-provider-enos", "the name of the provider")
	copyCmd.PersistentFlags().StringVar(&copyCfg.SrcProviderID, "src-provider-id", "hashicorp.com/qti/enos", "the name of the provider")
	copyCmd.PersistentFlags().StringVar(&copyCfg.DestProviderName, "dest-provider-name", "terraform-provider-enos", "the name of the provider")
	copyCmd.PersistentFlags().StringVar(&copyCfg.DestProviderID, "dest-provider-id", "hashicorp.com/qti/enos", "the name of the provider")

	_ = copyCmd.MarkFlagRequired("version")
	_ = copyCmd.MarkFlagRequired("src-bucket")
	_ = copyCmd.MarkFlagRequired("dest-bucket")
	_ = copyCmd.MarkFlagRequired("src-provider-name")
	_ = copyCmd.MarkFlagRequired("src-provider-id")
	_ = copyCmd.MarkFlagRequired("dest-provider-name")
	_ = copyCmd.MarkFlagRequired("dest-provider-id")

	return copyCmd
}

func runCopyCmd(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.requestTimeout)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	exitIfErr(err)

	copyCfg.SrcS3Client = s3.NewFromConfig(cfg)
	copyCfg.DestS3Client = s3.NewFromConfig(cfg)

	exitIfErr(publish.Copy(ctx, copyCfg))

	os.Exit(0)
}
