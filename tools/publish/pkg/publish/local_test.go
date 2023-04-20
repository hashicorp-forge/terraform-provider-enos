package publish

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAddGoBinariesFrom verifies that we can load binaries that are created
// by our build make targets.
func TestAddGoBinariesFrom(t *testing.T) {
	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

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

// TestAddGoBinariesFromWithRename verifies that we can load and rename binaries
func TestAddGoBinariesFromWithRename(t *testing.T) {
	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

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
