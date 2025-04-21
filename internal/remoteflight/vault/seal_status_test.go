// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSealStatusDeserialize(t *testing.T) {
	t.Parallel()

	expected := NewSealStatusResponse()
	expected.Data.BuildDate = "2023-04-28T20:01:42Z"
	expected.Data.ClusterID = "a009341f-5b9f-7166-a740-4aaaf0cedd27"
	expected.Data.ClusterName = "vault-cluster-50f6305a"
	expected.Data.Initialized = true
	expected.Data.Number = "5"
	expected.Data.Progress = "0"
	expected.Data.RecoverySeal = true
	expected.Data.StorageType = "raft"
	expected.Data.Threshold = "3"
	expected.Data.Type = "shamir"
	expected.Data.Version = "1.14.0-beta1+ent"

	got := NewSealStatusResponse()
	body := testReadSupport(t, "seal-status.json")
	require.NoError(t, json.Unmarshal(body, got))
	require.Equal(t, expected, got)
}

func TestSealStatusDeserializeOldResponseBody(t *testing.T) {
	t.Parallel()

	expected := NewSealStatusResponse()
	expected.Data.Number = "0"
	expected.Data.Progress = "0"
	expected.Data.Sealed = true
	expected.Data.StorageType = "raft"
	expected.Data.Threshold = "0"
	expected.Data.Type = "shamir"
	expected.Data.Version = "1.10.11"

	got := NewSealStatusResponse()
	body := testReadSupport(t, "seal-status-old.json")
	require.NoError(t, json.Unmarshal(body, got.Data))
	require.Equal(t, expected, got)
}
