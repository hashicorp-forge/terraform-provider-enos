package flightcontrol

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlightControlSupportedTarget(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		Platform      string
		Archictecture string
		Supported     bool
	}{
		{
			Platform:      "darwin",
			Archictecture: "amd64",
			Supported:     true,
		},
		{
			Platform:      "linux",
			Archictecture: "amd64",
			Supported:     true,
		},
		{
			Platform:      "linux",
			Archictecture: "386",
			Supported:     false,
		},
		{
			Platform:      "linux",
			Archictecture: "arm",
			Supported:     false,
		},
		{
			Platform:      "linux",
			Archictecture: "arm64",
			Supported:     false,
		},
		{
			Platform:      "freebsd",
			Archictecture: "386",
			Supported:     false,
		},
		{
			Platform:      "freebsd",
			Archictecture: "arm",
			Supported:     false,
		},
		{
			Platform:      "freebsd",
			Archictecture: "arm64",
			Supported:     false,
		},
		{
			Platform:      "windows",
			Archictecture: "amd64",
			Supported:     false,
		},
		{
			Platform:      "windows",
			Archictecture: "386",
			Supported:     false,
		},
	} {
		t.Run(fmt.Sprintf("%s_%s", test.Platform, test.Archictecture), func(t *testing.T) {
			supported, err := SupportedTarget(test.Platform, test.Archictecture)
			assert.Equal(t, test.Supported, supported)
			require.NoError(t, err)
		})
	}
}

func TestSupportedTargets(t *testing.T) {
	targets, err := SupportedTargets()
	require.NoError(t, err)
	require.EqualValues(t, map[string][]string{
		"darwin": {"amd64"},
		"linux":  {"amd64"},
	}, targets)
}
