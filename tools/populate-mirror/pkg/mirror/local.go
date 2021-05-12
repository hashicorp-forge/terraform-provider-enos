package mirror

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLocal takes a terraform provider name and returns a new local mirror
func NewLocal(name string) *Local {
	return &Local{
		providerName: name,
		mu:           sync.Mutex{},
		artifacts:    NewArtifacts(name),
		log:          zap.NewExample().Sugar(),
	}
}

// Local is a local provider artifact mirror
type Local struct {
	providerName string
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

// Close remoces all of the mirrors files
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
func (l *Local) LoadRemoteIndex(ctx context.Context, s3Client *s3.Client, bucket string, providerID string) error {
	return l.artifacts.LoadRemoteIndex(ctx, s3Client, bucket, providerID)
}

// PublishToRemoteBucket publishes the local mirror artifacts into the remote
// bucket.
func (l *Local) PublishToRemoteBucket(ctx context.Context, s3Client *s3.Client, bucket string, providerID string) error {
	return l.artifacts.PublishToRemoteBucket(ctx, s3Client, bucket, providerID)
}

// WriteMetadata writes metadata JSON files of the mirror artifacts
func (l *Local) WriteMetadata() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.artifacts.WriteMetadata()
}

// AddGoreleaserBinariesFrom takes a directory path to the goreleaser builds,
// walks it, finds any providers binaries, creates an archive of them and
// adds them to the artifacts and index.
func (l *Local) AddGoreleaserBinariesFrom(binPath string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var err error
	l.log.Infow("scanning for binaries",
		"path", binPath,
		"provider-name", l.providerName,
	)

	dirReg := regexp.MustCompile(l.providerName + `_(?P<platform>\w*)_(?P<arch>\w*)$`)
	binReg := regexp.MustCompile(l.providerName + `_(?P<version>.*)$`)
	binPath, err = filepath.Abs(binPath)
	if err != nil {
		return err
	}

	return filepath.Walk(binPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("accessing binary path: %w", err)
		}

		// If the path isn't a goreleaser terraform plugin output directory we
		// don't care about it
		if !info.IsDir() {
			return nil
		}

		l.log.Debugw("checking directory", "directory", info.Name())
		matches := dirReg.FindStringSubmatch(info.Name())
		if len(matches) != 3 {
			return nil
		}

		platform := matches[1]
		arch := matches[2]
		version := ""

		// Look in the release dir for a binary and get the version from its name
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				return err
			}

			matches = binReg.FindStringSubmatch(info.Name())
			if len(matches) != 2 {
				continue
			}

			version = matches[1]

			return l.artifacts.AddBinary(version, platform, arch, filepath.Join(path, entry.Name()))
		}

		return nil
	})
}
