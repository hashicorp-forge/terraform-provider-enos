package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"

	"github.com/hashicorp/enos-provider/tools/populate-mirror/pkg/mirror"
)

var distDir = flag.String("dist", "", "the output directory of goreleaser that build the artifacts")
var bucketPath = flag.String("bucket", "", "the S3 bucket path")
var providerName = flag.String("provider-name", "terraform-provider-enos", "the name of the provider")
var providerID = flag.String("provider-id", "hashicorp.com/qti/enos", "the name of the provider")
var timeout = flag.Duration("timeout", time.Duration(15*time.Minute), "maximum allowed time to run")
var level = zap.LevelFlag("log", zap.InfoLevel, "the log level (error, warn, info, debug, trace)")

func main() {
	parseFlags()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	exitIfErr(err)

	s3Client := s3.NewFromConfig(cfg)
	// Make sure the bucket exists and we have access to it before we attept
	// anything else
	_, err = s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(*bucketPath),
	})
	exitIfErr(err)

	mirror := mirror.NewLocal(*providerName)
	err = mirror.Initialize()
	exitIfErr(err)
	defer mirror.Close()

	exitIfErr(mirror.SetLogLevel(*level))

	exitIfErr(mirror.LoadRemoteIndex(ctx, s3Client, *bucketPath, *providerID))

	exitIfErr(mirror.AddGoreleaserBinariesFrom(*distDir))

	exitIfErr(mirror.WriteMetadata())

	exitIfErr(mirror.PublishToRemoteBucket(ctx, s3Client, *bucketPath, *providerID))

	os.Exit(0)
}

func parseFlags() {
	flag.Parse()

	if *distDir == "" {
		flag.Usage()
		exitIfErr(fmt.Errorf("you must provide a goreleaser artifact directory"))
	}

	if *bucketPath == "" {
		flag.Usage()
		exitIfErr(fmt.Errorf("you must provide an s3 bucket path"))
	}
}

func exitIfErr(err error) {
	if err != nil {
		fmt.Printf("ERROR: %s", err.Error())
		os.Exit(1)
	}
}
