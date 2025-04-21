// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
		Platform     string
		Architecture string
		Supported    bool
	}{
		{
			Platform:     "darwin",
			Architecture: "amd64",
			// https://github.com/upx/upx/issues/612 false while this is sorted out
			Supported: false,
		},
		{
			Platform:     "linux",
			Architecture: "amd64",
			Supported:    true,
		},
		{
			Platform:     "linux",
			Architecture: "386",
			Supported:    false,
		},
		{
			Platform:     "linux",
			Architecture: "arm",
			Supported:    false,
		},
		{
			Platform:     "linux",
			Architecture: "arm64",
			Supported:    true,
		},
		{
			Platform:     "linux",
			Architecture: "s390x",
			Supported:    true,
		},
		{
			Platform:     "freebsd",
			Architecture: "386",
			Supported:    false,
		},
		{
			Platform:     "freebsd",
			Architecture: "arm",
			Supported:    false,
		},
		{
			Platform:     "freebsd",
			Architecture: "arm64",
			Supported:    false,
		},
		{
			Platform:     "windows",
			Architecture: "amd64",
			Supported:    false,
		},
		{
			Platform:     "windows",
			Architecture: "386",
			Supported:    false,
		},
	} {
		t.Run(fmt.Sprintf("%s_%s", test.Platform, test.Architecture), func(t *testing.T) {
			t.Parallel()
			supported, err := SupportedTarget(test.Platform, test.Architecture)
			assert.Equal(t, test.Supported, supported)
			require.NoError(t, err)
		})
	}
}

func TestSupportedTargets(t *testing.T) {
	t.Parallel()
	targets, err := SupportedTargets()
	require.NoError(t, err)
	require.Equal(t, map[string][]string{
		"linux": {"amd64", "arm64", "s390x"},
	}, targets)
}
