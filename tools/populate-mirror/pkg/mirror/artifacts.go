package mirror

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	a.idx.Versions[version] = &IndexValue{}
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

	a.log.Infow(
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
