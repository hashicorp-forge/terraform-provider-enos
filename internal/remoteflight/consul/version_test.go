package consul

import (
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"
)

func TestParseConsulVersionInRange(t *testing.T) {
	r, err := semver.ParseRange(">= 1.10.3")
	require.NoError(t, err)

	for _, test := range []struct {
		version string
		inRange bool
	}{
		{"Consul v1.9.9", false},
		{"Consul v1.9.10", false},
		{"Consul v1.10.0-rc2", false},
		{"Consul v1.10.1", false},
		{"Consul v1.11.0-beta2+ent", true},
		{"Consul v1.10.3", true},
	} {
		t.Run(test.version, func(t *testing.T) {
			version, err := parseConsulVersion(test.version)
			require.NoError(t, err)

			if test.inRange {
				require.True(t, r(version))
			} else {
				require.False(t, r(version))
			}
		})
	}
}

func TestParseConsulVersion(t *testing.T) {
	for _, test := range []struct {
		version string
		major   uint64
		minor   uint64
		patch   uint64
		build   string
		pre     string
	}{
		{"Consul v1.9.9", 1, 9, 9, "", ""},
		{"Consul v1.9.10", 1, 9, 10, "", ""},
		{"Consul v1.10.0-rc2", 1, 10, 0, "", "rc2"},
		{"Consul v1.10.1", 1, 10, 1, "", ""},
		{"Consul v1.11.0-beta2+ent", 1, 11, 0, "ent", "beta2"},
		{"Consul v1.10.3", 1, 10, 3, "", ""},
	} {
		t.Run(test.version, func(t *testing.T) {
			version, err := parseConsulVersion(test.version)
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
