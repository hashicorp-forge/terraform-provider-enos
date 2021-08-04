package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/spf13/cobra"

	"github.com/hashicorp/enos-provider/tools/mirror/pkg/mirror"
)

type populateConfig struct {
	distDir      string
	bucketPath   string
	providerName string
	providerID   string
}

var populateCfg = &populateConfig{}

func newPopulateCommand() *cobra.Command {
	populateCmd := &cobra.Command{
		Use:   "populate --dist [DIR] --bucket [S3 BUCKETNAME]",
		Short: "populate the generated artifacts to remote S3 bucket",
		Run:   runPopulateCmd,
	}

	populateCmd.Flags().StringVar(&populateCfg.distDir, "dist", "", "the output directory of goreleaser that build the artifacts")
	populateCmd.Flags().StringVar(&populateCfg.bucketPath, "bucket", "", "the S3 bucket path")
	populateCmd.PersistentFlags().StringVar(&populateCfg.providerName, "provider-name", "terraform-provider-enos", "the name of the provider")
	populateCmd.PersistentFlags().StringVar(&populateCfg.providerID, "provider-id", "hashicorp.com/qti/enos", "the name of the provider")

	_ = populateCmd.MarkFlagRequired("distDir")
	_ = populateCmd.MarkFlagRequired("bucketPath")
	_ = populateCmd.MarkFlagRequired("providerName")
	_ = populateCmd.MarkFlagRequired("providerID")
	return populateCmd
}

func runPopulateCmd(cmd *cobra.Command, args []string) {
	if populateCfg.distDir == "" {
		fmt.Println("populate --dist [DIR] --bucket [S3 BUCKETNAME]")
		exitIfErr(fmt.Errorf("you must provide a goreleaser artifact directory"))
	}

	if populateCfg.bucketPath == "" {
		fmt.Println("populate --dist [DIR] --bucket [S3 BUCKETNAME]")
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
		Bucket: aws.String(populateCfg.bucketPath),
	})
	exitIfErr(err)

	mirror := mirror.NewLocal(populateCfg.providerName)
	err = mirror.Initialize()
	exitIfErr(err)
	defer mirror.Close()

	exitIfErr(mirror.SetLogLevel(*Level))

	exitIfErr(mirror.LoadRemoteIndex(ctx, s3Client, populateCfg.bucketPath, populateCfg.providerID))

	exitIfErr(mirror.AddGoreleaserBinariesFrom(populateCfg.distDir))

	exitIfErr(mirror.WriteMetadata())

	exitIfErr(mirror.PublishToRemoteBucket(ctx, s3Client, populateCfg.bucketPath, populateCfg.providerID))

	os.Exit(0)
}

func exitIfErr(err error) {
	if err != nil {
		fmt.Printf("ERROR: %s", err.Error())
		os.Exit(1)
	}
}
