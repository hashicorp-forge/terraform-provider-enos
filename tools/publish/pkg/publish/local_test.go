package publish

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
		assert.NoError(t, err)
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
		assert.NoError(t, err)
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

	err = promoteMirror.ExtractProviderBinaries(context.Background(), &TFCPromoteReq{
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
