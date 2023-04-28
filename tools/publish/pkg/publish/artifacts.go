package publish

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"go.uber.org/zap"
	"golang.org/x/mod/sumdb/dirhash"

	"github.com/hashicorp/enos-provider/internal/transport/file"
)

// NewArtifacts takes the name of the terraform provider and returns a new
// Artifacts.
func NewArtifacts(name string) *Artifacts {
	return &Artifacts{
		providerName: name,
		idx:          NewIndex(),
		releases:     map[string]*Release{},
		mu:           sync.Mutex{},
		log:          zap.NewExample().Sugar(),
		tfcMetadata:  map[string][]*TFCRelease{},
	}
}

// Artifacts is a collection of all the artifacts in the repository.
type Artifacts struct {
	providerName string
	idx          *Index
	releases     map[string]*Release // version -> release
	mu           sync.Mutex
	log          *zap.SugaredLogger
	dir          string
	tfcMetadata  map[string][]*TFCRelease // version -> []TFCRelease
}

// TFCRelease is a collection of TFC release zip binary, platform, architecture,
// and sha256sum.
type TFCRelease struct {
	Platform    string
	Arch        string
	ZipFilePath string
	SHA256Sum   string
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

// HashZipArchive returns the h1 style Terraform hash of the zip archive.
func (a *Artifacts) HashZipArchive(path string) (string, error) {
	return dirhash.HashZip(path, dirhash.Hash1)
}

// SHA256Sum returns the SHA256 sum of a file for a given path.
func (a *Artifacts) SHA256Sum(path string) (string, error) {
	f, err := file.Open(path)
	if err != nil {
		return "", err
	}

	bytes, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha256.Sum256(bytes)), nil
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

	sha256, err := a.SHA256Sum(zipFilePath)
	if err != nil {
		return err
	}

	archive := &Archive{
		URL:    zipFileName,
		Hashes: []string{hash},
	}

	release := &TFCRelease{
		Platform:    platform,
		Arch:        arch,
		ZipFilePath: zipFilePath,
		SHA256Sum:   sha256,
	}

	a.Insert(version, platform, arch, archive)
	a.InsertTFCRelease(version, release)

	return nil
}

// InsertTFCRelease takes a version, and release and inserts it into the
// tfcMetadata.
func (a *Artifacts) InsertTFCRelease(version string, release *TFCRelease) {
	a.mu.Lock()
	defer a.mu.Unlock()

	_, ok := a.tfcMetadata[version]
	if !ok {
		a.tfcMetadata[version] = []*TFCRelease{}
	}

	a.tfcMetadata[version] = append(a.tfcMetadata[version], release)
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

// WriteSHA256SUMS writes the release SHA256SUMS file as required by the TFC.
func (a *Artifacts) WriteSHA256SUMS(ctx context.Context, identityName string, sign bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.log.Infow(
		"writing SHA256SUMS file",
		"GPG signing", sign,
		"identity name", identityName,
		"artifacts dir", a.dir,
	)

	for version, releases := range a.tfcMetadata {
		shaPath := filepath.Join(a.dir, fmt.Sprintf("%s_SHA256SUMS", version))

		shaFile, err := os.OpenFile(shaPath, os.O_RDWR|os.O_CREATE, 0o755)
		if err != nil {
			return err
		}
		defer shaFile.Close()

		for _, release := range releases {
			a.log.Infow(
				"add zip SHA256 to SHASUMS file",
				"file", shaPath,
				"zip", filepath.Base(release.ZipFilePath),
			)
			_, err := fmt.Fprintf(shaFile, "%s %s\n", release.SHA256Sum, filepath.Base(release.ZipFilePath))
			if err != nil {
				return err
			}
		}

		if !sign {
			continue
		}

		sigPath := filepath.Join(a.dir, fmt.Sprintf("%s_SHA256SUMS.sig", version))
		err = a.WriteDetachedSignature(ctx, shaPath, sigPath, identityName)
		if err != nil {
			return err
		}
	}

	return nil
}

// WriteDetachedSignature takes a context, source file path, outfile path, and the GPG entity name and
// writes a signed version of the source file to the outfile.
func (a *Artifacts) WriteDetachedSignature(ctx context.Context, source, out, name string) error {
	a.log.Infow(
		"writing detached signature",
		"source_file", source,
		"out_file", out,
		"ident_name", name,
	)

	return exec.CommandContext(ctx, "gpg", "--detach-sign", "--local-user", name, source).Run()
}

// PublishToTFC publishes the artifact version to TFC org.
func (a *Artifacts) PublishToTFC(ctx context.Context, tfcreq *TFCUploadReq) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.log.Infow(
		"publishing to TFC private provider",
	)

	tfcclient, err := NewTFCClient(WithTFCToken(tfcreq.TFCToken), WithTFCOrg(tfcreq.TFCOrg), WithTFCLog(a.log))
	if err != nil {
		return err
	}

	err = tfcclient.FindOrCreateProvider(ctx, tfcreq.TFCOrg, tfcreq.ProviderName)
	if err != nil {
		return err
	}

	for version, releases := range a.tfcMetadata {
		providerVersion := version
		sha256sumsPath := filepath.Join(a.dir, fmt.Sprintf("%s_SHA256SUMS", providerVersion))

		err = tfcclient.FindOrCreateVersion(ctx, tfcreq.TFCOrg, tfcreq.ProviderName, providerVersion, tfcreq.GPGKeyID, sha256sumsPath)
		if err != nil {
			return err
		}
		err = tfcclient.FindOrCreatePlatform(ctx, tfcreq.TFCOrg, tfcreq.ProviderName, providerVersion, releases)
		if err != nil {
			return err
		}
	}

	return err
}

// DownloadFromTFC downloads the artifacts for a given version to a directory.
func (a *Artifacts) DownloadFromTFC(ctx context.Context, tfcreq *TFCDownloadReq) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.log.Infow(
		"Downloading from TFC private provider",
	)

	tfcclient, err := NewTFCClient(WithTFCToken(tfcreq.TFCToken), WithTFCOrg(tfcreq.TFCOrg), WithTFCLog(a.log))
	if err != nil {
		return err
	}

	// Empty the Downloads directory if it already exists
	downloadir, err := filepath.Abs(tfcreq.DownloadDir)
	if err != nil {
		return err
	}
	err = os.RemoveAll(downloadir)
	if err != nil {
		return err
	}

	platforms, err := tfcclient.FindProviderPlatform(ctx, tfcreq.TFCOrg, tfcreq.ProviderName, tfcreq.ProviderVersion)
	if err != nil {
		return fmt.Errorf("error finding provider platform %w", err)
	}
	if len(platforms) == 0 {
		return fmt.Errorf("no data found for provider platform %w", err)
	}

	for i := range platforms {
		filesha := platforms[i].SHAsum
		filename := platforms[i].Filename
		url := platforms[i].PlatformBinaryURL

		if _, err := os.Stat(tfcreq.DownloadDir); err != nil {
			if os.IsNotExist(err) {
				err := os.Mkdir(tfcreq.DownloadDir, 0o755)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}

		err = tfcclient.downloadFile(ctx, tfcreq.DownloadDir, filename, url)
		if err != nil {
			return err
		}

		downloadedfile := filepath.Join(tfcreq.DownloadDir, filename)
		downloadedsha, err := a.SHA256Sum(downloadedfile)
		if err != nil {
			return err
		}

		if downloadedsha != filesha {
			return fmt.Errorf("download failed: unxpected SHA 256 sum: expected (%s) received (%s)", filesha, downloadedsha)
		}
	}

	return err
}

// ExtractProviderBinaries extracts the downloaded artifacts to an output directory.
func (a *Artifacts) ExtractProviderBinaries(ctx context.Context, tfcreq *TFCPromoteReq) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.log.Infow(
		"extracting TFC private provider binaries",
	)

	archiveDir, err := filepath.Abs(tfcreq.DownloadsDir)
	if err != nil {
		return err
	}

	promoteDir, err := filepath.Abs(tfcreq.PromoteDir)
	if err != nil {
		return err
	}

	if _, err := os.Stat(promoteDir); err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(promoteDir, 0o755)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return filepath.Walk(archiveDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("accessing archive path: %w", err)
		}

		// Our directory should be filled with archives with the following name
		// schema: terraform-provider-enos_0.3.24_darwin_amd64.zip
		// Each archive should contain a single file with a matching binary.
		// Extract all of them into the "promote" dir.
		if info.IsDir() {
			return nil
		}

		a.log.Debugw("found file", "file", info.Name())

		if !strings.HasSuffix(info.Name(), ".zip") {
			return fmt.Errorf("unexpected provider archive file name %s", info.Name())
		}

		parts := strings.Split(info.Name(), "_")
		if len(parts) != 4 {
			return fmt.Errorf("unexpected provider archive file name %s", info.Name())
		}

		if parts[0] != tfcreq.SrcBinaryName {
			return fmt.Errorf("unexpected provider archive name, expected %s, found %s", tfcreq.SrcBinaryName, parts[0])
		}

		zipArchive, err := zip.OpenReader(filepath.Join(archiveDir, info.Name()))
		if err != nil {
			return fmt.Errorf("%w: opening zip archive", err)
		}
		defer zipArchive.Close()

		if len(zipArchive.File) != 1 {
			return fmt.Errorf("unexpected provider archive file contents. Expected 1 file, got %d files", len(zipArchive.File))
		}

		zippedBinFile := zipArchive.File[0]
		zippedBin, err := zippedBinFile.Open()
		if err != nil {
			return fmt.Errorf("%w: opening zipped binary in zip archive", err)
		}
		defer zippedBin.Close()

		binPath := filepath.Join(promoteDir, zippedBinFile.Name)
		binFile, err := os.OpenFile(binPath, os.O_RDWR|os.O_CREATE, zippedBinFile.Mode())
		if err != nil {
			return fmt.Errorf("%w: unzipping binary from archive", err)
		}
		defer binFile.Close()

		if _, err := io.Copy(binFile, zippedBin); err != nil {
			return err
		}

		return nil
	})
}

// LoadRemoteIndex fetches the existing index.json from the remote bucket and loads
// it. This way we can merge new builds with those in existing remote mirror.
func (a *Artifacts) LoadRemoteIndex(ctx context.Context, s3Client *s3.Client, bucket, providerID string) error {
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
		//nolint:nilerr// it's okay to return nil if the index doesn't exist
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
func (a *Artifacts) LoadRemoteReleaseMetedataForVersion(ctx context.Context, s3Client *s3.Client, bucket, providerID, version string) error {
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
// a remote s3 mirror to another remote mirror.
func (a *Artifacts) CopyReleaseArtifactsBetweenRemoteBucketsForVersion(ctx context.Context, srcBucketName string, destS3Client *s3.Client, destBucketName, providerName, providerID, version string) error {
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

// AddReleaseVersionToIndex adds the release version to the mirror index.
func (a *Artifacts) AddReleaseVersionToIndex(version string) {
	a.idx.Versions[version] = &IndexValue{}
}

// PublishToRemoteBucket publishes the artifacts in the local mirror to the
// remote S3 Bucket.
func (a *Artifacts) PublishToRemoteBucket(ctx context.Context, s3Client *s3.Client, bucket, providerID string) error {
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
