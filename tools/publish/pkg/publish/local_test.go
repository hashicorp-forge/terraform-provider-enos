// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package publish

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestAddGoBinariesFrom verifies that we can load binaries that are created
// by our build make targets.
func TestAddGoBinariesFrom(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	var err error

	for _, bin := range []string{
		"terraform-provider-enos_0.3.24_darwin_amd64",
		"terraform-provider-enos_0.3.24_darwin_arm64",
		"terraform-provider-enos_0.3.24_linux_amd64",
		"terraform-provider-enos_0.3.24_linux_arm64",
	} {
		err = os.WriteFile(filepath.Join(dir, bin), []byte("something"), 0o755)
		require.NoError(t, err)
	}

	mirror := NewLocal("enos", "terraform-provider-enos")
	err = mirror.Initialize()
	require.NoError(t, err)

	err = mirror.AddGoBinariesFrom(dir)
	require.NoError(t, err)

	for _, zipName := range []string{
		"terraform-provider-enos_0.3.24_linux_amd64.zip",
		"terraform-provider-enos_0.3.24_linux_arm64.zip",
		"terraform-provider-enos_0.3.24_darwin_amd64.zip",
		"terraform-provider-enos_0.3.24_darwin_arm64.zip",
	} {
		_, err = os.Open(filepath.Join(mirror.artifacts.dir, zipName))
		require.NoError(t, err)
	}
}

// TestAddGoBinariesFromWithRename verifies that we can load and rename binaries.
func TestAddGoBinariesFromWithRename(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	var err error

	for _, bin := range []string{
		"terraform-provider-enos_0.3.24_darwin_amd64",
		"terraform-provider-enos_0.3.24_darwin_arm64",
		"terraform-provider-enos_0.3.24_linux_amd64",
		"terraform-provider-enos_0.3.24_linux_arm64",
	} {
		err = os.WriteFile(filepath.Join(dir, bin), []byte("something"), 0o755)
		require.NoError(t, err)
	}

	mirror := NewLocal(
		"enos",
		"terraform-provider-enos",
		WithLocalBinaryRename("terraform-provider-enosdev"),
	)
	err = mirror.Initialize()
	require.NoError(t, err)

	err = mirror.AddGoBinariesFrom(dir)
	require.NoError(t, err)

	for _, zipName := range []string{
		"terraform-provider-enosdev_0.3.24_linux_amd64.zip",
		"terraform-provider-enosdev_0.3.24_linux_arm64.zip",
		"terraform-provider-enosdev_0.3.24_darwin_amd64.zip",
		"terraform-provider-enosdev_0.3.24_darwin_arm64.zip",
	} {
		_, err = os.Open(filepath.Join(mirror.artifacts.dir, zipName))
		require.NoError(t, err)
	}
}

// TestProviderBinariesRoundTrip does a round trip of creating provider
// binaries with the standard name, creating a local archive set with a rename,
// then unarchiving them with a rename. This simulates the whole create and
// rename process that is used during our upload to dev -> promote to prod
// pipeline.
func TestProviderBinariesRoundTrip(t *testing.T) {
	t.Parallel()

	var err error

	// where our source "binaries" live
	distDir := t.TempDir()
	// where our unzipped "binaries" live before promotion
	promoteDir := t.TempDir()

	// Create fake source binaries
	for _, bin := range []string{
		"terraform-provider-enos_0.3.24_darwin_amd64",
		"terraform-provider-enos_0.3.24_darwin_arm64",
		"terraform-provider-enos_0.3.24_linux_amd64",
		"terraform-provider-enos_0.3.24_linux_arm64",
	} {
		err = os.WriteFile(filepath.Join(distDir, bin), []byte("something"), 0o755)
		require.NoError(t, err)
	}

	// Add source binaries and pretend to upload them to the "enosdev" registry
	uploadMirror := NewLocal(
		"enos",
		"terraform-provider-enos",
		WithLocalBinaryRename("terraform-provider-enosdev"),
	)
	err = uploadMirror.Initialize()
	require.NoError(t, err)

	err = uploadMirror.AddGoBinariesFrom(distDir)
	require.NoError(t, err)

	for _, zipName := range []string{
		"terraform-provider-enosdev_0.3.24_linux_amd64.zip",
		"terraform-provider-enosdev_0.3.24_linux_arm64.zip",
		"terraform-provider-enosdev_0.3.24_darwin_amd64.zip",
		"terraform-provider-enosdev_0.3.24_darwin_arm64.zip",
	} {
		// Verify that our archives are created
		zFile, err := os.Open(filepath.Join(uploadMirror.artifacts.dir, zipName))
		require.NoError(t, err)
		_ = zFile.Close()
	}

	// Create our promotion mirror and extract our "downloaded" enosdev archives
	// into "enos" binary artifacts in the promtion dir
	promoteMirror := NewLocal(
		"enos",
		"terraform-provider-enosdev",
		WithLocalBinaryRename("terraform-provider-enos"),
	)
	err = promoteMirror.Initialize()
	require.NoError(t, err)

	err = promoteMirror.ExtractProviderBinaries(t.Context(), &TFCPromoteReq{
		ProviderVersion:  "0.3.24",
		DownloadsDir:     uploadMirror.artifacts.dir,
		PromoteDir:       promoteDir,
		SrcProviderName:  "enosdev",
		DestProviderName: "enos",
		SrcBinaryName:    "terraform-provider-enosdev",
		DestBinaryName:   "terraform-provider-enos",
	})
	require.NoError(t, err)

	// Verify that they're extracted in the promotion dir but still have their
	// renamed binaries. The rename happens during the upload.
	for _, bin := range []string{
		"terraform-provider-enosdev_0.3.24_darwin_amd64",
		"terraform-provider-enosdev_0.3.24_darwin_arm64",
		"terraform-provider-enosdev_0.3.24_linux_amd64",
		"terraform-provider-enosdev_0.3.24_linux_arm64",
	} {
		sF, err := os.Open(filepath.Join(distDir, strings.ReplaceAll(bin, "enosdev", "enos")))
		require.NoError(t, err)
		sfC, err := io.ReadAll(sF)
		require.NoError(t, err)
		dF, err := os.Open(filepath.Join(promoteDir, bin))
		require.NoError(t, err)
		dfC, err := io.ReadAll(dF)
		require.NoError(t, err)
		require.Equal(t, sfC, dfC)
	}
}

// TestAddGoBinariesFrom verifies that the local mirror will have all of the required files
// for a public release.
// See https://developer.hashicorp.com/terraform/registry/providers/publishing#manually-preparing-a-release
func TestPublicRegistry(t *testing.T) {
	t.Parallel()

	gpgident := os.Getenv("TEST_GPG_IDENTITY_NAME")
	if gpgident == "" {
		t.Skip("skipping as TEST_GPG_IDENTITY_NAME environement variable is not set to a gpg identity")
	}

	// Allow a lot of time to handle GPG passphrase pin entry
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	dir := t.TempDir()
	var err error

	// Create our local mirror
	mirror := NewLocal("enos", "terraform-provider-enos")
	err = mirror.Initialize()
	require.NoError(t, err)

	// Add our registry manifest
	content := `{"version":1,"metadata":{"protocol_versions":["6.0"]}}`
	path := filepath.Join(dir, "manifest.json")
	require.NoError(t, os.WriteFile(path, []byte(content), fs.FileMode(0o755)))
	require.NoError(t, mirror.AddReleaseManifest(ctx, path, "0.5.0"))

	// Start tracking our expected SHA256SUMS
	expectedShas := map[string][32]byte{
		"terraform-provider-enos_0.5.0_manifest.json": sha256.Sum256([]byte(content)),
	}

	// Add our binaries
	for _, bin := range []string{
		"terraform-provider-enos_0.5.0_darwin_amd64",
		"terraform-provider-enos_0.5.0_darwin_arm64",
		"terraform-provider-enos_0.5.0_linux_amd64",
		"terraform-provider-enos_0.5.0_linux_arm64",
	} {
		err = os.WriteFile(filepath.Join(dir, bin), []byte(bin), 0o755)
		require.NoError(t, err)
		f, err := os.Open(filepath.Join(dir, bin))
		require.NoError(t, err)
		bytes, err := io.ReadAll(f)
		require.NoError(t, err)
		expectedShas[bin+".zip"] = sha256.Sum256(bytes)
	}

	err = mirror.AddGoBinariesFrom(dir)
	require.NoError(t, err)

	// Create a manifest
	require.NoError(t, mirror.CreateVersionedRegistryManifest(ctx))

	// Write the SHA256Sums
	require.NoError(t, mirror.WriteSHA256Sums(ctx, RegistryTypePublic, gpgident, true))

	// Make sure the mirror has all the expected files
	for _, blob := range []string{
		"terraform-provider-enos_0.5.0_linux_amd64.zip",
		"terraform-provider-enos_0.5.0_linux_arm64.zip",
		"terraform-provider-enos_0.5.0_darwin_amd64.zip",
		"terraform-provider-enos_0.5.0_darwin_arm64.zip",
		"terraform-provider-enos_0.5.0_SHA256SUMS",
		"terraform-provider-enos_0.5.0_SHA256SUMS.sig",
		"terraform-provider-enos_0.5.0_manifest.json",
	} {
		_, err = os.Stat(filepath.Join(mirror.artifacts.dir, blob))
		if err != nil {
			t.Logf("did not find file  matching '%s' in local mirror", blob)
			files, err1 := os.ReadDir(mirror.artifacts.dir)
			require.NoError(t, err1)
			t.Logf("found files %v+", files)
			t.FailNow()
		}
	}

	// Make sure our SHASUMS has a line entry for each blob and that it has a matching SHA256SUM
	sums, err := os.Open(filepath.Join(mirror.artifacts.dir, "terraform-provider-enos_0.5.0_SHA256SUMS"))
	require.NoError(t, err)
	sbytes, err := io.ReadAll(sums)
	require.NoError(t, err)
LOOP:
	for name, sha256 := range expectedShas {
		scanner := bufio.NewScanner(bytes.NewBuffer(sbytes))
		for scanner.Scan() {
			parts := strings.SplitN(scanner.Text(), " ", 2)
			if len(parts) != 2 {
				continue
			}

			if parts[1] == name && parts[0] == hex.EncodeToString(sha256[:]) {
				break LOOP
			}
		}
		require.NoError(t, scanner.Err())
		t.Logf("did not find line matching '%s %s' in SHA256SUMS", hex.EncodeToString(sha256[:]), name)
		t.Logf("content of file:\n%s", string(sbytes))
		t.FailNow()
	}
}
