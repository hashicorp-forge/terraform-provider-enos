// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHealthDeserialize(t *testing.T) {
	t.Parallel()

	expected := NewHealthResponse()
	expected.License.State = "autoloaded"
	expected.License.ExpiryTime = "2026-07-01T00:00:00Z"
	expected.ReplicationDRMode = "unknown"
	expected.ReplicationPerformanceMode = "unknown"
	expected.Sealed = true
	expected.ServerTimeUTC = 1683834751
	expected.Standby = true
	expected.Version = "1.14.0-beta1+ent"

	got := NewHealthResponse()
	body := testReadSupport(t, "health.json")
	require.NoError(t, json.Unmarshal(body, got))
	require.Equal(t, expected, got)
}
