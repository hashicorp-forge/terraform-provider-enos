// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigStateSanitizedDeserialize(t *testing.T) {
	t.Parallel()
	expected := NewConfigStateSanitizedResponse()
	expected.Data.APIAddr = "http://10.13.10.150:8200"
	expected.Data.CacheSize = "0"
	expected.Data.ClusterAddr = "http://10.13.10.150:8201"
	expected.Data.DefaultLeaseTTL = "0"
	expected.Data.DefaultMaxRequestDuration = "0"
	expected.Data.EnableUI = true
	expected.Data.Listeners = []*ConfigListener{
		{
			Config: &ConfigListenerConfig{
				Address:    "0.0.0.0:8200",
				TLSDisable: "true",
			},
			Type: "tcp",
		},
	}
	expected.Data.LogLevel = "info"
	expected.Data.MaxLeaseTTL = "0"
	expected.Data.Seals = []*ConfigSeals{
		{
			Disabled: false,
			Type:     "awskms",
		},
	}
	expected.Data.Storage = &ConfigStorage{
		ClusterAddr:  "http://10.13.10.150:8201",
		RedirectAddr: "http://10.13.10.150:8200",
		Type:         "raft",
	}

	got := NewConfigStateSanitizedResponse()
	body := testReadSupport(t, "config-sanitized.json")
	require.NoError(t, json.Unmarshal(body, got))

	require.Equal(t, expected, got)
}
