// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package systemd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnitPropertiesHasPropertiesEnabledAndRunning(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		props      func() UnitProperties
		shouldFail bool
	}{
		"enabled-and-running": {
			testReadShow(t, "show-consul.service"),
			false,
		},
		"not-enabled-and-running": {
			testReadShow(t, "show-nope.service"),
			true,
		},
		"not-loaded": {
			func() UnitProperties {
				return UnitProperties{
					"LoadState":   "active-found",
					"ActiveState": "inactive",
					"SubState":    "dead",
				}
			},
			true,
		},
		"not-active": {
			func() UnitProperties {
				return UnitProperties{
					"LoadState":   "loaded",
					"ActiveState": "inactive",
					"SubState":    "dead",
				}
			},
			true,
		},
		"not-running": {
			func() UnitProperties {
				return UnitProperties{
					"LoadState":   "loaded",
					"ActiveState": "active",
					"SubState":    "dead",
				}
			},
			true,
		},
		"not-enabled": {
			func() UnitProperties {
				return UnitProperties{
					"LoadState":   "loaded",
					"ActiveState": "active",
					"SubState":    "running",
				}
			},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if test.shouldFail {
				require.False(t, test.props().HasProperties(EnabledAndRunningProperties))
			} else {
				require.True(t, test.props().HasProperties(EnabledAndRunningProperties))
			}
		})
	}
}

func TestUnitPropertiesFind(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		props      func() UnitProperties
		names      []string
		shouldFail bool
	}{
		"has-props": {
			testReadShow(t, "show-consul.service"),
			[]string{"IgnoreSIGPIPE", "Nice"},
			false,
		},
		"does-not-have-props": {
			testReadShow(t, "show-nope.service"),
			[]string{"InvocationID", "ExecStart"},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, found := test.props().Find(test.names...)
			if test.shouldFail {
				require.Error(t, found)
			} else {
				require.NoError(t, found)
			}
		})
	}
}

func TestUnitPropertiesFindProperties(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		props      func() UnitProperties
		want       UnitProperties
		shouldFail bool
	}{
		"has-props": {
			testReadShow(t, "show-consul.service"),
			UnitProperties{
				"IgnoreSIGPIPE": "yes",
				"Nice":          "0",
			},
			false,
		},
		"does-not-have-props": {
			testReadShow(t, "show-nope.service"),
			UnitProperties{
				"InvocationID": "861eaf48e60d450fafdabf560855f7d8",
				"ExecStart":    "{ path=/opt/consul/bin/consul ; argv[]=/opt/consul/bin/consul agent -config-dir /etc/consul.d/consul.hcl ; ignore_errors=no ; start_time=[n/a] ; stop_time=[n/a] ; pid=0 ; code=(null) ; status=0/0 }",
			},
			true,
		},
	} {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, found := test.props().FindProperties(test.want)
			if test.shouldFail {
				require.Error(t, found)
			} else {
				require.NoError(t, found)
			}
		})
	}
}

func testReadShow(t *testing.T, name string) func() UnitProperties {
	t.Helper()

	return func() UnitProperties {
		p, err := filepath.Abs(filepath.Join("./support", name))
		require.NoError(t, err)
		content, err := os.ReadFile(p)
		require.NoError(t, err)
		props, err := decodeUnitPropertiesFromShow(string(content))
		require.NoError(t, err)

		return props
	}
}
