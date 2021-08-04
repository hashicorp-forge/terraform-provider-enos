package mirror

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"go.uber.org/zap/zapcore"
)

// PromoteReq to be copied
type PromoteReq struct {
	Version          string
	SrcBucketName    string
	DestBucketName   string
	SrcProviderName  string
	SrcProviderID    string
	DestProviderName string
	DestProviderID   string
	SrcS3Client      *s3.Client
	DestS3Client     *s3.Client
	log              zapcore.Level
}

// Promote will promote the artifact from source to destination S3 bucket
func Promote(ctx context.Context, req *PromoteReq) error {
	// Make sure the source and destination buckets exist
	_, err := req.SrcS3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(req.SrcBucketName),
	})
	if err != nil {
		return err
	}

	_, err = req.DestS3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(req.DestBucketName),
	})
	if err != nil {
		return err
	}

	// Initialize our source mirror and verify that the it has the version that
	// we want to promote
	srcMirror := NewLocal(req.SrcProviderName)
	err = srcMirror.Initialize()
	if err != nil {
		return err
	}
	defer srcMirror.Close()

	err = srcMirror.SetLogLevel(req.log)
	if err != nil {
		return err
	}

	err = srcMirror.LoadRemoteIndex(ctx, req.SrcS3Client, req.SrcBucketName, req.SrcProviderID)
	if err != nil {
		return err
	}

	ok, err := srcMirror.HasVersion(ctx, req.Version)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("version not found")
	}

	// Initialize our destination mirror and make sure it doesn't already have
	// the version
	destMirror := NewLocal(req.DestProviderName)
	err = destMirror.Initialize()
	if err != nil {
		return err
	}
	defer destMirror.Close()

	err = destMirror.SetLogLevel(req.log)
	if err != nil {
		return err
	}

	err = destMirror.LoadRemoteIndex(ctx, req.DestS3Client, req.DestBucketName, req.DestProviderID)
	if err != nil {
		return err
	}

	ok, err = destMirror.HasVersion(ctx, req.Version)
	if err != nil {
		return err
	}
	if ok {
		return fmt.Errorf("version already promoted")
	}

	// Copy artifacts from the source S3 mirror to destination S3 mirror
	err = srcMirror.CopyReleaseArtifactsBetweenRemoteBucketsForVersion(ctx, req.SrcBucketName, req.DestS3Client, req.DestBucketName, req.SrcProviderName, req.SrcProviderID, req.Version)
	if err != nil {
		return err
	}

	// Add the version we copied to our desitnation mirror release index and
	// upload it.
	destMirror.AddReleaseVersionToIndex(ctx, req.Version)

	err = destMirror.WriteMetadata()
	if err != nil {
		return err
	}

	return destMirror.PublishToRemoteBucket(ctx, req.DestS3Client, req.DestBucketName, req.DestProviderID)
}
