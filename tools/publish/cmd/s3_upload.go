package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/spf13/cobra"

	"github.com/hashicorp/enos-provider/tools/publish/pkg/publish"
)

// UploadReq for upload
type UploadReq struct {
	distDir      string
	bucketPath   string
	providerName string
	binaryName   string
	providerID   string
}

var uploadCfg = &UploadReq{}

func news3UploadCmd() *cobra.Command {
	uploadCmd := &cobra.Command{
		Use:   "upload --dist [DIR] --bucket [S3 BUCKETNAME]",
		Short: "upload the generated artifacts to remote S3 bucket",
		Run:   runUploadCmd,
	}

	uploadCmd.Flags().StringVar(&uploadCfg.distDir, "dist", "", "the output directory of goreleaser that build the artifacts")
	uploadCmd.Flags().StringVar(&uploadCfg.bucketPath, "bucket", "", "the S3 bucket path")
	uploadCmd.PersistentFlags().StringVar(&uploadCfg.providerName, "provider-name", "terraform-provider-enos", "the name of the provider")
	uploadCmd.PersistentFlags().StringVar(&uploadCfg.binaryName, "binary-name", "terraform-provider-enos", "the name of the provider binary")
	uploadCmd.PersistentFlags().StringVar(&uploadCfg.providerID, "provider-id", "hashicorp.com/qti/enos", "the name of the provider")

	_ = uploadCmd.MarkFlagRequired("distDir")
	_ = uploadCmd.MarkFlagRequired("bucketPath")
	_ = uploadCmd.MarkFlagRequired("providerName")
	_ = uploadCmd.MarkFlagRequired("providerID")
	return uploadCmd
}

func runUploadCmd(cmd *cobra.Command, args []string) {
	if uploadCfg.distDir == "" {
		fmt.Println("upload --dist [DIR] --bucket [S3 BUCKETNAME]")
		exitIfErr(fmt.Errorf("you must provide a goreleaser artifact directory"))
	}

	if uploadCfg.bucketPath == "" {
		fmt.Println("upload --dist [DIR] --bucket [S3 BUCKETNAME]")
		exitIfErr(fmt.Errorf("you must provide an s3 bucket path"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.requestTimeout)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	exitIfErr(err)

	s3Client := s3.NewFromConfig(cfg)
	// Make sure the bucket exists and we have access to it before we attempt
	// anything else
	_, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(uploadCfg.bucketPath),
	})
	exitIfErr(err)

	publish := publish.NewLocal(uploadCfg.providerName, uploadCfg.binaryName)
	err = publish.Initialize()
	exitIfErr(err)
	defer publish.Close()

	exitIfErr(publish.SetLogLevel(*Level))

	exitIfErr(publish.LoadRemoteIndex(ctx, s3Client, uploadCfg.bucketPath, uploadCfg.providerID))

	exitIfErr(publish.AddGoreleaserBinariesFrom(uploadCfg.distDir))

	exitIfErr(publish.WriteMetadata())

	exitIfErr(publish.PublishToRemoteBucket(ctx, s3Client, uploadCfg.bucketPath, uploadCfg.providerID))

	os.Exit(0)
}

func exitIfErr(err error) {
	if err != nil {
		fmt.Printf("ERROR: %s", err.Error())
		os.Exit(1)
	}
}
