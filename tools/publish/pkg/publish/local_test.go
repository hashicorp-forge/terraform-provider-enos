package publish

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleGo118AMD64VersionZipName verifies that we can handle goreleaser
// directories that are created for ARM64 binaries with Go 1.18 (the release
// that added support for ARM64 architecture optimizations
// https://github.com/golang/go/wiki/MinimumRequirements#amd64
// Instead of carrying along the architecture version we throw it away in our
// provider mirror artifacts.
func TestHandleGo118AMD64VersionZipName(t *testing.T) {
	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	binName := "terraform-provider-enos_0.1.23"
	for _, subDir := range []string{
		"terraform-provider-enos_linux_amd64_v1",
		"terraform-provider-enos_linux_arm64",
		"terraform-provider-enos_darwin_amd64_v1",
		"terraform-provider-enos_darwin_arm64",
	} {
		d := filepath.Join(dir, subDir)
		err = os.MkdirAll(d, 0o755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(d, binName), []byte("something"), 0o755)
		require.NoError(t, err)
	}

	mirror := NewLocal("terraform-provider-enos")
	err = mirror.Initialize()
	require.NoError(t, err)

	err = mirror.AddGoreleaserBinariesFrom(dir)
	require.NoError(t, err)

	for _, zipName := range []string{
		"terraform-provider-enos_0.1.23_linux_amd64.zip",
		"terraform-provider-enos_0.1.23_linux_arm64.zip",
		"terraform-provider-enos_0.1.23_darwin_amd64.zip",
		"terraform-provider-enos_0.1.23_darwin_arm64.zip",
	} {
		_, err = os.Open(filepath.Join(mirror.artifacts.dir, zipName))
		assert.NoError(t, err)
	}
}
