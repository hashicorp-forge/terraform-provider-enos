// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package publish

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateZipArchiveFilePermissions(t *testing.T) {
	t.Parallel()

	modes := []fs.FileMode{
		fs.FileMode(0o666),
		fs.FileMode(0o523),
		fs.FileMode(0o556),
		fs.FileMode(0o712),
	}

	dir := t.TempDir()
	var err error
	artifacts := NewArtifacts("test")

	for _, mode := range modes {
		binPath := filepath.Join(dir, "bin-"+mode.String())
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
}

func TestReleaseManifestVersionRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	var err error
	artifacts := NewArtifacts("test")
	artifacts.dir = dir
	content := `{"version":1,"metadata":{"protocol_versions":["6.0"]}}`
	path := filepath.Join(dir, "manifest.json")
	require.NoError(t, os.WriteFile(path, []byte(content), fs.FileMode(0o755)))
	require.NoError(t, artifacts.AddReleaseManifest(context.Background(), path, "0.5.0"))
	require.NoError(t, artifacts.CreateVersionedRegistryManifest(context.Background()))
	f, err := os.Open(artifacts.registryManifest.versionedPath)
	require.NoError(t, err)
	got, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, content, string(got))
}
