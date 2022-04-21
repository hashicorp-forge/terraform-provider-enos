package publish

import (
	"archive/zip"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateZipArchiveFilePermissions(t *testing.T) {
	modes := []fs.FileMode{
		fs.FileMode(0o666),
		fs.FileMode(0o523),
		fs.FileMode(0o556),
		fs.FileMode(0o712),
	}

	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	artifacts := NewArtifacts("test")

	for _, mode := range modes {
		binPath := filepath.Join(dir, fmt.Sprintf("bin-%s", mode.String()))
		err = os.WriteFile(binPath, []byte("something"), mode)
		require.NoError(t, err)
		err = os.Chmod(binPath, mode)
		require.NoError(t, err)

		s, err := os.Stat(binPath)
		require.NoError(t, err)
		require.Equal(t, s.Mode(), mode)

		zipPath := filepath.Join(dir, fmt.Sprintf("test-%s.zip", mode.String()))
		require.NoError(t, artifacts.CreateZipArchive(binPath, zipPath))

		zip, err := zip.OpenReader(zipPath)
		require.NoError(t, err)

		assert.Equal(t, mode.String(), zip.File[0].FileHeader.Mode().String())
	}
	t.Log(dir)
}
