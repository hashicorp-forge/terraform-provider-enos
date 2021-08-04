package main

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"

	"github.com/hashicorp/enos-provider/tools/mirror/pkg/mirror"
)

var promoteCfg = &mirror.PromoteReq{}

func newPromoteCommand() *cobra.Command {
	// promoteCmd represents the promote command
	promoteCmd := &cobra.Command{
		Use:   "promote --version [ARTIFACT_VERSION] --src-bucket [S3 BUCKETNAME] --dest-bucket [S3 BUCKETNAME]",
		Short: "promote the artifacts with given version from remote source S3 mirror to destination S3 mirror",
		Run:   runPromoteCmd,
	}

	promoteCmd.Flags().StringVar(&promoteCfg.Version, "version", "", "the version of the artifact to be promoted")
	promoteCmd.Flags().StringVar(&promoteCfg.SrcBucketName, "src-bucket", "", "the Source S3 bucket name")
	promoteCmd.Flags().StringVar(&promoteCfg.DestBucketName, "dest-bucket", "", "the Destination S3 bucket name")
	promoteCmd.PersistentFlags().StringVar(&promoteCfg.SrcProviderName, "src-provider-name", "terraform-provider-enos", "the name of the provider")
	promoteCmd.PersistentFlags().StringVar(&promoteCfg.SrcProviderID, "src-provider-id", "hashicorp.com/qti/enos", "the name of the provider")
	promoteCmd.PersistentFlags().StringVar(&promoteCfg.DestProviderName, "dest-provider-name", "terraform-provider-enos", "the name of the provider")
	promoteCmd.PersistentFlags().StringVar(&promoteCfg.DestProviderID, "dest-provider-id", "hashicorp.com/qti/enos", "the name of the provider")

	_ = promoteCmd.MarkFlagRequired("version")
	_ = promoteCmd.MarkFlagRequired("src-bucket")
	_ = promoteCmd.MarkFlagRequired("dest-bucket")
	_ = promoteCmd.MarkFlagRequired("src-provider-name")
	_ = promoteCmd.MarkFlagRequired("src-provider-id")
	_ = promoteCmd.MarkFlagRequired("dest-provider-name")
	_ = promoteCmd.MarkFlagRequired("dest-provider-id")

	return promoteCmd
}

func runPromoteCmd(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.requestTimeout)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	exitIfErr(err)

	promoteCfg.SrcS3Client = s3.NewFromConfig(cfg)
	promoteCfg.DestS3Client = s3.NewFromConfig(cfg)

	exitIfErr(mirror.Promote(ctx, promoteCfg))

	os.Exit(0)
}
