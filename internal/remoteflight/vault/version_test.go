package vault

import (
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"
)

func TestParseVaultVersionInRange(t *testing.T) {
	r, err := semver.ParseRange(">= 1.8.0-rc1")
	require.NoError(t, err)

	for _, test := range []struct {
		version string
		inRange bool
	}{
		{"Vault v1.4.0+ent", false},
		{"Vault v1.4.0", false},
		{"Vault v1.5.0+ent", false},
		{"Vault v1.5.0", false},
		{"Vault v1.6.0-rc+ent (c2d9da7229dc4159ab5ad8e04834460980060ad4)", false},
		{"Vault v1.6.0 (7ce0bd9691998e0443bc77e98b1e2a4ab1e965d4)", false},
		{"Vault v1.6.5+ent (945069754dfb1b6cc1c972a56fee8fe2d2d30ef2)", false},
		{"Vault v1.7.0-rc2+ent (eba403741e344f9d9b686eaca122a5a3f446d442)", false},
		{"Vault v1.7.2+ent (0e2ed0c723715690ce84695805e84e00466909c0)", false},
		{"Vault v1.7.2 (db0e4245d5119b5929e611ea4d9bf66e47f3f208)", false},
		{"Vault v1.7.0-rc2+ent.hsm (eba403741e344f9d9b686eaca122a5a3f446d442)", false},
		{"Vault v1.8.0-rc1 (eba403741e344f9d9b686eaca122a5a3f446d442)", true},
		{"Vault v1.8.0-rc1+ent.hsm (eba403741e344f9d9b686eaca122a5a3f446d442)", true},
	} {
		t.Run(test.version, func(t *testing.T) {
			version, err := parseVaultVersion(test.version)
			require.NoError(t, err)

			if test.inRange {
				require.True(t, r(version))
			} else {
				require.False(t, r(version))
			}
		})
	}
}

func TestParseVaultVersion(t *testing.T) {
	for _, test := range []struct {
		version string
		major   uint64
		minor   uint64
		patch   uint64
		build   string
		pre     string
	}{
		{"Vault v1.4.0+ent", 1, 4, 0, "ent", ""},
		{"Vault v1.5.0", 1, 5, 0, "", ""},
		{"Vault v1.6.5+ent (945069754dfb1b6cc1c972a56fee8fe2d2d30ef2)", 1, 6, 5, "ent", ""},
		{"Vault v1.7.0-rc2+ent.hsm (eba403741e344f9d9b686eaca122a5a3f446d442)", 1, 7, 0, "ent.hsm", "rc2"},
		{"Vault v1.8.0-rc1 (eba403741e344f9d9b686eaca122a5a3f446d442)", 1, 8, 0, "", "rc1"},
		{"Vault v1.8.0-rc1+ent.hsm (eba403741e344f9d9b686eaca122a5a3f446d442)", 1, 8, 0, "ent.hsm", "rc1"},
	} {
		t.Run(test.version, func(t *testing.T) {
			version, err := parseVaultVersion(test.version)
			require.NoError(t, err)

			require.Equal(t, test.major, version.Major)
			require.Equal(t, test.minor, version.Minor)
			require.Equal(t, test.patch, version.Patch)
			if test.build != "" {
				require.Equal(t, strings.Split(test.build, "."), version.Build)
			} else {
				require.Empty(t, version.Build)
			}
			if test.pre != "" {
				// NOTE: this assumes there is only one pre-release
				require.Equal(t, test.pre, version.Pre[0].VersionStr)
			} else {
				require.Empty(t, version.Pre)
			}
		})
	}
}
