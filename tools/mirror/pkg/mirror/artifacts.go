package mirror

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"go.uber.org/zap"
	"golang.org/x/mod/sumdb/dirhash"
)

// NewArtifacts takes the name of the terraform provider and returns a new
// Artifacts
func NewArtifacts(name string) *Artifacts {
	return &Artifacts{
		providerName: name,
		idx:          NewIndex(),
		releases:     map[string]*Release{},
		mu:           sync.Mutex{},
		log:          zap.NewExample().Sugar(),
	}
}

// Artifacts is a collection of all the artifacts in the repository.
type Artifacts struct {
	providerName string
	idx          *Index
	releases     map[string]*Release // platform_arch
	mu           sync.Mutex
	log          *zap.SugaredLogger
	dir          string
}

// CreateZipArchive takes a source binary and a destination path and creates
// a zip archive of the binary.
func (a *Artifacts) CreateZipArchive(sourceBinaryPath, zipFilePath string) error {
	a.log.Infow("creating zip archive",
		"source", sourceBinaryPath,
		"destination", zipFilePath,
	)

	zipFile, err := os.OpenFile(zipFilePath, os.O_RDWR|os.O_CREATE, 0o755)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	sourceFile, err := os.Open(sourceBinaryPath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	zipHeader, err := zip.FileInfoHeader(sourceInfo)
	if err != nil {
		return err
	}
	zipHeader.Method = zip.Deflate
	zipHeader.Modified = sourceInfo.ModTime()
	zipHeader.SetMode(sourceInfo.Mode())

	zipper := zip.NewWriter(zipFile)
	binZip, err := zipper.CreateHeader(zipHeader)
	if err != nil {
		return err
	}

	_, err = io.Copy(binZip, sourceFile)
	if err != nil {
		return err
	}

	return zipper.Close()
}

// HashZipArchive returns the h1 style Terraform hash of the zip archive
func (a *Artifacts) HashZipArchive(path string) (string, error) {
	return dirhash.HashZip(path, dirhash.Hash1)
}

// AddBinary takes version, platform, arch, and path of a binary and adds it to
// the mirror. It does this by creating a zip archive of the binary, hashing it
// and adding it to the artifacts collection.
func (a *Artifacts) AddBinary(version, platform, arch, binaryPath string) error {
	zipFileName := fmt.Sprintf("%s_%s_%s_%s.zip", a.providerName, version, platform, arch)
	zipFilePath := filepath.Join(a.dir, zipFileName)

	err := a.CreateZipArchive(binaryPath, zipFilePath)
	if err != nil {
		return err
	}

	hash, err := a.HashZipArchive(zipFilePath)
	if err != nil {
		return err
	}

	archive := &Archive{
		URL:    zipFileName,
		Hashes: []string{hash},
	}

	a.Insert(version, platform, arch, archive)

	return nil
}

// Insert takes a version, platform, arch, and archive and inserts it into the
// archive collection.
func (a *Artifacts) Insert(version, platform, arch string, archive *Archive) {
	a.mu.Lock()
	defer a.mu.Unlock()

	_, ok := a.releases[version]
	if !ok {
		release := NewRelease()
		a.releases[version] = release
	}

	a.releases[version].AddArchive(platform, arch, archive)
	a.AddReleaseVersionToIndex(version)
}

// WriteMetadata writes the artifact collection as JSON metadata into the local
// mirror.
func (a *Artifacts) WriteMetadata() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	path := filepath.Join(a.dir, "index.json")
	idx, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o755)
	if err != nil {
		return err
	}

	err = a.idx.AsJSON(idx)
	if err != nil {
		return err
	}

	for version, release := range a.releases {
		path := filepath.Join(a.dir, fmt.Sprintf("%s.json", version))
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o755)
		if err != nil {
			return err
		}

		err = release.AsJSON(file)
		if err != nil {
			return err
		}
	}

	return nil
}

// LoadRemoteIndex fetches the existing index.json from the remote bucket and loads
// it. This way we can merge new builds with those in existing remote mirror.
func (a *Artifacts) LoadRemoteIndex(ctx context.Context, s3Client *s3.Client, bucket string, providerID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.log.Debugw(
		"attempting to get remote mirror index.json from bucket",
		"bucket", bucket,
		"id", providerID,
	)

	idxKey := aws.String(filepath.Join(providerID, "index.json"))
	head, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    idxKey,
	})
	if err != nil {
		a.log.Warn("index.json does not exist, this could be the first time we're running in this bucket")
		return nil
	}

	buf := make([]byte, int(head.ContentLength))
	writer := manager.NewWriteAtBuffer(buf)
	downloader := manager.NewDownloader(s3Client)
	_, err = downloader.Download(ctx, writer, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    idxKey,
	})
	if err != nil {
		return err
	}

	a.idx = &Index{}

	return json.Unmarshal(buf, a.idx)
}

// LoadRemoteReleaseMetedataForVersion fetches the version.json for a given
// release from the remote bucket and loads it. We need this information to
// known what and where release artifacts for the release are.
func (a *Artifacts) LoadRemoteReleaseMetedataForVersion(ctx context.Context, s3Client *s3.Client, bucket string, providerID string, version string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := aws.String(filepath.Join(providerID, fmt.Sprintf("%s.json", version)))

	a.log.Debugw(
		"attempting to get remote mirror file from bucket",
		"bucket", bucket,
		"id", providerID,
		"key", key,
	)

	head, err := s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    key,
	})
	if err != nil {
		return err
	}

	buf := make([]byte, int(head.ContentLength))
	bufwriter := manager.NewWriteAtBuffer(buf)
	downloader := manager.NewDownloader(s3Client)
	_, err = downloader.Download(ctx, bufwriter, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    key,
	})
	if err != nil {
		return err
	}

	a.releases[version] = NewRelease()

	return json.Unmarshal(buf, a.releases[version])
}

// HasVersion checks if the artifact version exists in the loaded index.json
// of the remote mirror.
func (a *Artifacts) HasVersion(ctx context.Context, version string) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.log.Debugw("checking if version exists",
		"version", version,
	)

	if a.idx == nil {
		return false, fmt.Errorf("no index")
	}

	_, ok := a.idx.Versions[version]

	return ok, nil
}

// CopyReleaseArtifactsBetweenRemoteBucketsForVersion copies artifacts from
// a remote s3 mirror to another remote mirror
func (a *Artifacts) CopyReleaseArtifactsBetweenRemoteBucketsForVersion(ctx context.Context, srcBucketName string, destS3Client *s3.Client, destBucketName string, providerName string, providerID string, version string) error {
	a.log.Infow("copying release artifacts between remote buckets", "source bucket", srcBucketName, "destination bucket", destBucketName)

	err := a.LoadRemoteReleaseMetedataForVersion(ctx, destS3Client, srcBucketName, providerID, version)
	if err != nil {
		return err
	}

	filesToCopy := map[string]string{
		path.Join(srcBucketName, providerID, fmt.Sprintf("%s.json", version)): path.Join(providerID, fmt.Sprintf("%s.json", version)),
	}
	for _, archive := range a.releases[version].Archives {
		filesToCopy[path.Join(srcBucketName, providerID, archive.URL)] = path.Join(providerID, archive.URL)
	}

	for srcFile, destKey := range filesToCopy {
		a.log.Debugw("copying file",
			"bucket", destBucketName,
			"source", srcFile,
			"destination", destKey,
		)

		input := &s3.CopyObjectInput{
			Bucket:     aws.String(destBucketName),
			CopySource: aws.String(srcFile),
			Key:        aws.String(destKey),
			// Todo:
			// ACL:        aws.String("bucket-owner-full-control"),
		}

		_, err := destS3Client.CopyObject(ctx, input)
		if err != nil {
			return err
		}

	}

	return nil
}

// AddReleaseVersionToIndex adds the release version to the mirror index
func (a *Artifacts) AddReleaseVersionToIndex(version string) {
	a.idx.Versions[version] = &IndexValue{}
}

// PublishToRemoteBucket publishes the artifacts in the local mirror to the
// the remote S3 Bucket.
func (a *Artifacts) PublishToRemoteBucket(ctx context.Context, s3Client *s3.Client, bucket string, providerID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.log.Infow("publishing local mirror to remote bucket", "bucket", bucket)

	uploader := manager.NewUploader(s3Client)

	entries, err := os.ReadDir(a.dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// This should never happen
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		f, err := os.Open(filepath.Join(a.dir, info.Name()))
		if err != nil {
			return err
		}

		key := filepath.Join(providerID, info.Name())
		input := &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   f,
		}

		switch filepath.Ext(info.Name()) {
		case ".zip":
			input.ContentType = aws.String("application/zip")
		case ".json":
			input.ContentType = aws.String("application/json")
		}

		a.log.Debugw("uploading file to bucket",
			"bucket", bucket,
			"file", key,
		)

		_, err = uploader.Upload(ctx, input)
		if err != nil {
			return err
		}
	}

	return nil
}
