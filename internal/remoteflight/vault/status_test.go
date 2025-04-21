// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatusDeserialize(t *testing.T) {
	t.Parallel()
	expected := NewStatusResponse()
	expected.SealType = "shamir"
	expected.Initialized = true
	expected.HAEnabled = true
	expected.Version = "1.14.0-beta1+ent"

	got := NewStatusResponse()
	body := testReadSupport(t, "status.json")
	require.NoError(t, json.Unmarshal(body, got))
	require.Equal(t, expected, got)
}
