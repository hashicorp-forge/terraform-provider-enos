package publish

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLocal takes a terraform provider name and returns a new local mirror
func NewLocal(name, binname string, opts ...NewLocalOpt) *Local {
	l := &Local{
		providerName: name,
		binaryName:   binname,
		mu:           sync.Mutex{},
		artifacts:    NewArtifacts(binname),
		log:          zap.NewExample().Sugar(),
	}

	for _, opt := range opts {
		l = opt(l)
	}

	if l.binaryRename == "" {
		l.artifacts = NewArtifacts(binname)
	}

	return l
}

// NewLocalOpt accepts optional arguments for Local
type NewLocalOpt func(*Local) *Local

// WithLocalBinaryRename renames the binary during creation
func WithLocalBinaryRename(name string) NewLocalOpt {
	return func(l *Local) *Local {
		l.binaryRename = name
		l.artifacts = NewArtifacts(name)
		return l
	}
}

// Local is a local provider artifact mirror
type Local struct {
	providerName string
	binaryName   string
	binaryRename string
	artifacts    *Artifacts
	mu           sync.Mutex
	log          *zap.SugaredLogger
}

// Initialize initializes the mirror
func (l *Local) Initialize() error {
	l.log.Debug("intializing")

	l.mu.Lock()
	defer l.mu.Unlock()

	err := l.SetLogLevel(zap.ErrorLevel)
	if err != nil {
		return err
	}

	l.artifacts.dir, err = os.MkdirTemp("", "local-mirror")
	if err != nil {
		return err
	}
	l.log.Debug("created temporary directory for local mirror", "directory", l.artifacts.dir)

	return err
}

// Close removes all of the mirrors files
func (l *Local) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	return os.RemoveAll(l.artifacts.dir)
}

// SetLogLevel sets the log level
func (l *Local) SetLogLevel(level zapcore.Level) error {
	cfg := zap.NewDevelopmentConfig()
	cfg.Level.SetLevel(level)
	logger, err := cfg.Build()
	if err != nil {
		return err
	}

	l.log = logger.Sugar()
	l.artifacts.log = l.log

	return nil
}

// LoadRemoteIndex fetches a remote index and loads it
func (l *Local) LoadRemoteIndex(ctx context.Context, s3Client *s3.Client, bucket, providerID string) error {
	return l.artifacts.LoadRemoteIndex(ctx, s3Client, bucket, providerID)
}

// HasVersion checks if t the version exists in the bucket
func (l *Local) HasVersion(ctx context.Context, version string) (bool, error) {
	return l.artifacts.HasVersion(ctx, version)
}

// CopyReleaseArtifactsBetweenRemoteBucketsForVersion copies release artifacts from source bucket
// to the destination bucket.
func (l *Local) CopyReleaseArtifactsBetweenRemoteBucketsForVersion(ctx context.Context, srcBucketName string, destS3Client *s3.Client, destBucketName, binaryName, providerID, version string) error {
	return l.artifacts.CopyReleaseArtifactsBetweenRemoteBucketsForVersion(ctx, srcBucketName, destS3Client, destBucketName, binaryName, providerID, version)
}

// AddReleaseVersionToIndex adds a version to the release index
func (l *Local) AddReleaseVersionToIndex(ctx context.Context, version string) {
	l.artifacts.AddReleaseVersionToIndex(version)
}

// PublishToRemoteBucket publishes the local mirror artifacts into the remote
// bucket.
func (l *Local) PublishToRemoteBucket(ctx context.Context, s3Client *s3.Client, bucket, providerID string) error {
	return l.artifacts.PublishToRemoteBucket(ctx, s3Client, bucket, providerID)
}

// WriteMetadata writes metadata JSON files of the mirror artifacts
func (l *Local) WriteMetadata() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.artifacts.WriteMetadata()
}

// WriteSHA256Sums writes a detached signature of the source file to the outfile
func (l *Local) WriteSHA256Sums(ctx context.Context, name string, sign bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.artifacts.WriteSHA256SUMS(ctx, name, sign)
}

// PublishToTFC publishes artifact version to TFC org
func (l *Local) PublishToTFC(ctx context.Context, tfcreq *TFCUploadReq) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.artifacts.PublishToTFC(ctx, tfcreq)
}

// DownloadFromTFC downloads the artifacts for a given version to a direcotry
func (l *Local) DownloadFromTFC(ctx context.Context, tfcreq *TFCDownloadReq) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.artifacts.DownloadFromTFC(ctx, tfcreq)
}

// ExtractProviderBinaries extracts the artifacts to an output directory
func (l *Local) ExtractProviderBinaries(ctx context.Context, tfcreq *TFCPromoteReq) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.artifacts.ExtractProviderBinaries(ctx, tfcreq)
}

// AddGoBinariesFrom takes a directory path to the go builds, walks it, finds
// any providers binaries, creates an archive of them and adds them to the
// artifacts and index.
func (l *Local) AddGoBinariesFrom(binPath string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var err error
	l.log.Infow("scanning for binaries",
		"path", binPath,
		"provider-binary-name", l.binaryName,
		"provider-binary-rename", l.binaryRename,
	)

	binPath, err = filepath.Abs(binPath)
	if err != nil {
		return err
	}

	var renameTempDir string
	if l.binaryRename != "" {
		var err error
		renameTempDir, err = os.MkdirTemp("", "rename-temp-dir")
		if err != nil {
			return err
		}
		defer os.RemoveAll(renameTempDir)
	}

	return filepath.Walk(binPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("accessing binary path: %w", err)
		}

		// Our directory should be filled with binaries with the following name
		// schema: terraform-provider-enos_0.3.24_darwin_amd64
		if info.IsDir() {
			return nil
		}

		l.log.Debugw("found file", "file", info.Name())

		parts := strings.Split(info.Name(), "_")
		if len(parts) != 4 {
			l.log.Debugw("skipping file as it does not appear to be a provider binary", "file", info.Name())
			return nil
		}

		if parts[0] != l.binaryName {
			l.log.Debugw("skipping file as it does not appear to be a provider binary", "file", info.Name())
			return nil
		}

		binaryName := parts[0]
		version := parts[1]
		platform := parts[2]
		arch := parts[3]
		releasePath := path

		l.log.Debugw("found provider binary",
			"binary_name", binaryName,
			"version", version,
			"platform", platform,
			"arch", arch,
			"file_path", releasePath,
			"file", info.Name(),
		)

		if l.binaryRename != "" {
			newPath := filepath.Join(renameTempDir, strings.Join([]string{
				l.binaryRename, version, platform, arch,
			}, "_"))

			l.log.Infow("renaming provider binary",
				"original_name", binaryName,
				"new_name", l.binaryRename,
				"original_path", releasePath,
				"new_path", newPath,
				"version", version,
				"platform", platform,
				"arch", arch,
				"file", info.Name(),
			)

			newFile, err := os.Create(newPath)
			if err != nil {
				return err
			}

			err = os.Chmod(newFile.Name(), 0o755)
			if err != nil {
				return err
			}

			sourceFile, err := os.Open(releasePath)
			if err != nil {
				return err
			}

			// Copying the files to another directory to preserve the source dist directory
			_, err = io.Copy(newFile, sourceFile)
			if err != nil {
				return err
			}

			releasePath = newPath
		}

		return l.artifacts.AddBinary(version, platform, arch, releasePath)
	})
}
