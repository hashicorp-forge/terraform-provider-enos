// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestVaultStorageConfigRoundtripNoRetryJoin(t *testing.T) {
	t.Parallel()

	storageType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"type":       tftypes.String,
		"attributes": tftypes.Object{AttributeTypes: map[string]tftypes.Type{"path": tftypes.String}},
	}}

	storageVal := tftypes.NewValue(storageType, map[string]tftypes.Value{
		"type": tftypes.NewValue(tftypes.String, "raft"),
		"attributes": tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"path": tftypes.String,
		}}, map[string]tftypes.Value{
			"path": tftypes.NewValue(tftypes.String, "/opt/raft"),
		}),
	})

	cfg := newVaultStorageConfig()
	require.NoError(t, cfg.FromTerraform5Value(storageVal))
	require.Empty(t, cfg.RetryJoins.Val, "expected RetryJoins.Val to be empty when no retry_join configured")

	// RetryJoin should also be empty when no retry_join configured
	retryJoinObj, ok := cfg.RetryJoin.Object.GetObject()
	if ok {
		require.Empty(t, retryJoinObj, "expected RetryJoin.Object to be empty when no retry_join configured")
	}

	got := cfg.Terraform5Value()
	require.True(t, got.Equal(storageVal), "expected storage config without retry_join to round-trip unchanged")
}

func TestVaultStorageConfigRountripListOfObjects(t *testing.T) {
	t.Parallel()

	retryJoinType := tftypes.List{
		ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}},
	}

	retryJoinVal := tftypes.NewValue(retryJoinType, []tftypes.Value{
		tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}}, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-0:8200"),
		}),
		tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}}, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-1:8200"),
		}),
	})

	storageType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"type":       tftypes.String,
		"attributes": tftypes.Object{AttributeTypes: map[string]tftypes.Type{"path": tftypes.String}},
		"retry_join": retryJoinType,
	}}

	storageVal := tftypes.NewValue(storageType, map[string]tftypes.Value{
		"type": tftypes.NewValue(tftypes.String, "raft"),
		"attributes": tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"path": tftypes.String,
		}}, map[string]tftypes.Value{
			"path": tftypes.NewValue(tftypes.String, "/opt/raft"),
		}),
		"retry_join": retryJoinVal,
	})

	cfg := newVaultStorageConfig()
	require.NoError(t, cfg.FromTerraform5Value(storageVal))
	require.NotEmpty(t, cfg.RetryJoins.Val, "expected RetryJoins.Val to be populated for Render() to use")
	require.Len(t, cfg.RetryJoins.Val, 2, "expected 2 retry_join blocks")

	got := cfg.Terraform5Value()
	require.True(t, got.Equal(storageVal), "expected storage config to preserve homogeneous retry_join list")
}

func TestVaultStorageConfigRetryJoinRoundtripListOfTuples(t *testing.T) {
	t.Parallel()

	retryJoinType := tftypes.Tuple{
		ElementTypes: []tftypes.Type{
			tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"auto_join":        tftypes.String,
				"auto_join_scheme": tftypes.String,
			}},
			tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"leader_api_addr": tftypes.String,
			}},
			tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"leader_api_addr": tftypes.String,
			}},
		},
	}

	retryJoinVal := tftypes.NewValue(retryJoinType, []tftypes.Value{
		tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"auto_join":        tftypes.String,
			"auto_join_scheme": tftypes.String,
		}}, map[string]tftypes.Value{
			"auto_join":        tftypes.NewValue(tftypes.String, "provider=aws tag_key=Type tag_value=vault"),
			"auto_join_scheme": tftypes.NewValue(tftypes.String, "https"),
		}),
		tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}}, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-1:8200"),
		}),
		tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}}, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-2:8200"),
		}),
	})

	storageType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"type":       tftypes.String,
		"attributes": tftypes.Object{AttributeTypes: map[string]tftypes.Type{"path": tftypes.String}},
		"retry_join": retryJoinType,
	}}

	storageVal := tftypes.NewValue(storageType, map[string]tftypes.Value{
		"type": tftypes.NewValue(tftypes.String, "raft"),
		"attributes": tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"path": tftypes.String,
		}}, map[string]tftypes.Value{
			"path": tftypes.NewValue(tftypes.String, "/opt/raft"),
		}),
		"retry_join": retryJoinVal,
	})

	cfg := newVaultStorageConfig()
	require.NoError(t, cfg.FromTerraform5Value(storageVal))
	require.NotEmpty(t, cfg.RetryJoins.Val, "expected RetryJoins.Val to be populated for Render() to use")
	require.Len(t, cfg.RetryJoins.Val, 3, "expected 3 retry_join blocks")

	got := cfg.Terraform5Value()
	require.True(t, got.Equal(storageVal), "expected storage config to preserve heterogeneous retry_join tuple")
}

func TestVaultStorageConfigRetryJoinRoundtripSingleObject(t *testing.T) {
	t.Parallel()

	retryJoinType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"leader_api_addr": tftypes.String,
	}}

	storageType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"type":       tftypes.String,
		"attributes": tftypes.Object{AttributeTypes: map[string]tftypes.Type{"path": tftypes.String}},
		"retry_join": retryJoinType,
	}}

	storageVal := tftypes.NewValue(storageType, map[string]tftypes.Value{
		"type": tftypes.NewValue(tftypes.String, "raft"),
		"attributes": tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"path": tftypes.String,
		}}, map[string]tftypes.Value{
			"path": tftypes.NewValue(tftypes.String, "/opt/raft"),
		}),
		"retry_join": tftypes.NewValue(retryJoinType, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-0:8200"),
		}),
	})

	cfg := newVaultStorageConfig()
	require.NoError(t, cfg.FromTerraform5Value(storageVal))

	got := cfg.Terraform5Value()
	require.True(t, got.Equal(storageVal), "expected single retry_join block to round-trip unchanged")
}

func TestVaultStorageConfigRetryJoinSingleElementListObject(t *testing.T) {
	t.Parallel()

	retryJoinType := tftypes.List{
		ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}},
	}

	retryJoinVal := tftypes.NewValue(retryJoinType, []tftypes.Value{
		tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}}, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-0:8200"),
		}),
	})

	storageType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"type":       tftypes.String,
		"attributes": tftypes.Object{AttributeTypes: map[string]tftypes.Type{"path": tftypes.String}},
		"retry_join": retryJoinType,
	}}

	storageVal := tftypes.NewValue(storageType, map[string]tftypes.Value{
		"type": tftypes.NewValue(tftypes.String, "raft"),
		"attributes": tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"path": tftypes.String,
		}}, map[string]tftypes.Value{
			"path": tftypes.NewValue(tftypes.String, "/opt/raft"),
		}),
		"retry_join": retryJoinVal,
	})

	cfg := newVaultStorageConfig()
	require.NoError(t, cfg.FromTerraform5Value(storageVal))
	require.NotEmpty(t, cfg.RetryJoins.Val, "expected RetryJoins.Val to be populated for Render() to use")
	require.Len(t, cfg.RetryJoins.Val, 1, "expected 1 retry_join block")

	got := cfg.Terraform5Value()
	require.True(t, got.Equal(storageVal), "expected storage config to preserve single element list")
}

func TestVaultStorageConfigRetryJoinSingleElementListTuple(t *testing.T) {
	t.Parallel()

	retryJoinType := tftypes.Tuple{
		ElementTypes: []tftypes.Type{
			tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"leader_api_addr": tftypes.String,
			}},
		},
	}

	retryJoinVal := tftypes.NewValue(retryJoinType, []tftypes.Value{
		tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}}, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-0:8200"),
		}),
	})

	storageType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"type":       tftypes.String,
		"attributes": tftypes.Object{AttributeTypes: map[string]tftypes.Type{"path": tftypes.String}},
		"retry_join": retryJoinType,
	}}

	storageVal := tftypes.NewValue(storageType, map[string]tftypes.Value{
		"type": tftypes.NewValue(tftypes.String, "raft"),
		"attributes": tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"path": tftypes.String,
		}}, map[string]tftypes.Value{
			"path": tftypes.NewValue(tftypes.String, "/opt/raft"),
		}),
		"retry_join": retryJoinVal,
	})

	cfg := newVaultStorageConfig()
	require.NoError(t, cfg.FromTerraform5Value(storageVal))
	require.NotEmpty(t, cfg.RetryJoins.Val, "expected RetryJoins.Val to be populated for Render() to use")
	require.Len(t, cfg.RetryJoins.Val, 1, "expected 1 retry_join block")

	got := cfg.Terraform5Value()
	require.True(t, got.Equal(storageVal), "expected storage config to preserve single element tuple")
}

func TestVaultStorageConfigRetryJoinTwoElementListObject(t *testing.T) {
	t.Parallel()

	retryJoinType := tftypes.List{
		ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}},
	}

	retryJoinVal := tftypes.NewValue(retryJoinType, []tftypes.Value{
		tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}}, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-0:8200"),
		}),
		tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}}, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-1:8200"),
		}),
	})

	storageType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"type":       tftypes.String,
		"attributes": tftypes.Object{AttributeTypes: map[string]tftypes.Type{"path": tftypes.String}},
		"retry_join": retryJoinType,
	}}

	storageVal := tftypes.NewValue(storageType, map[string]tftypes.Value{
		"type": tftypes.NewValue(tftypes.String, "raft"),
		"attributes": tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"path": tftypes.String,
		}}, map[string]tftypes.Value{
			"path": tftypes.NewValue(tftypes.String, "/opt/raft"),
		}),
		"retry_join": retryJoinVal,
	})

	cfg := newVaultStorageConfig()
	require.NoError(t, cfg.FromTerraform5Value(storageVal))
	require.NotEmpty(t, cfg.RetryJoins.Val, "expected RetryJoins.Val to be populated for Render() to use")
	require.Len(t, cfg.RetryJoins.Val, 2, "expected 2 retry_join blocks")

	got := cfg.Terraform5Value()
	require.True(t, got.Equal(storageVal), "expected storage config to preserve two element list")
}

func TestVaultStorageConfigRetryJoinTwoElementListTuple(t *testing.T) {
	t.Parallel()

	retryJoinType := tftypes.Tuple{
		ElementTypes: []tftypes.Type{
			tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"leader_api_addr": tftypes.String,
			}},
			tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"leader_api_addr": tftypes.String,
			}},
		},
	}

	retryJoinVal := tftypes.NewValue(retryJoinType, []tftypes.Value{
		tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}}, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-0:8200"),
		}),
		tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}}, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-1:8200"),
		}),
	})

	storageType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"type":       tftypes.String,
		"attributes": tftypes.Object{AttributeTypes: map[string]tftypes.Type{"path": tftypes.String}},
		"retry_join": retryJoinType,
	}}

	storageVal := tftypes.NewValue(storageType, map[string]tftypes.Value{
		"type": tftypes.NewValue(tftypes.String, "raft"),
		"attributes": tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"path": tftypes.String,
		}}, map[string]tftypes.Value{
			"path": tftypes.NewValue(tftypes.String, "/opt/raft"),
		}),
		"retry_join": retryJoinVal,
	})

	cfg := newVaultStorageConfig()
	require.NoError(t, cfg.FromTerraform5Value(storageVal))
	require.NotEmpty(t, cfg.RetryJoins.Val, "expected RetryJoins.Val to be populated")
	require.Len(t, cfg.RetryJoins.Val, 2, "expected 2 retry_join blocks")

	got := cfg.Terraform5Value()
	require.True(t, got.Equal(storageVal), "expected storage config to preserve two element tuple")
}

func TestVaultStorageConfigSingleRetryJoinGetObject(t *testing.T) {
	t.Parallel()

	retryJoinType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"leader_api_addr": tftypes.String,
	}}

	storageType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"type":       tftypes.String,
		"attributes": tftypes.Object{AttributeTypes: map[string]tftypes.Type{"path": tftypes.String}},
		"retry_join": retryJoinType,
	}}

	storageVal := tftypes.NewValue(storageType, map[string]tftypes.Value{
		"type": tftypes.NewValue(tftypes.String, "raft"),
		"attributes": tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"path": tftypes.String,
		}}, map[string]tftypes.Value{
			"path": tftypes.NewValue(tftypes.String, "/opt/raft"),
		}),
		"retry_join": tftypes.NewValue(retryJoinType, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-0:8200"),
		}),
	})

	cfg := newVaultStorageConfig()
	require.NoError(t, cfg.FromTerraform5Value(storageVal))

	// Verify GetObject() can access the single retry_join via RetryJoin.Object
	retryJoinAttrs, ok := cfg.RetryJoin.Object.GetObject()
	require.True(t, ok, "expected RetryJoin.GetObject() to have value")
	require.NotNil(t, retryJoinAttrs)
	require.Contains(t, retryJoinAttrs, "leader_api_addr")
	require.Equal(t, "http://vault-0:8200", retryJoinAttrs["leader_api_addr"])
}

func TestVaultStorageConfigRetryJoinMultiObjectGetObject(t *testing.T) {
	t.Parallel()

	retryJoinType := tftypes.List{
		ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}},
	}

	retryJoinVal := tftypes.NewValue(retryJoinType, []tftypes.Value{
		tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}}, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-0:8200"),
		}),
		tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"leader_api_addr": tftypes.String,
		}}, map[string]tftypes.Value{
			"leader_api_addr": tftypes.NewValue(tftypes.String, "http://vault-1:8200"),
		}),
	})

	storageType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"type":       tftypes.String,
		"attributes": tftypes.Object{AttributeTypes: map[string]tftypes.Type{"path": tftypes.String}},
		"retry_join": retryJoinType,
	}}

	storageVal := tftypes.NewValue(storageType, map[string]tftypes.Value{
		"type": tftypes.NewValue(tftypes.String, "raft"),
		"attributes": tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{
			"path": tftypes.String,
		}}, map[string]tftypes.Value{
			"path": tftypes.NewValue(tftypes.String, "/opt/raft"),
		}),
		"retry_join": retryJoinVal,
	})

	cfg := newVaultStorageConfig()
	require.NoError(t, cfg.FromTerraform5Value(storageVal))

	// Verify Get() can access multiple retry_joins via RetryJoins.Get()
	retryJoins, ok := cfg.RetryJoins.Get()
	require.True(t, ok, "expected RetryJoins to be populated")
	require.Len(t, retryJoins, 2)

	// Check first retry_join
	attrs0, ok := retryJoins[0].GetObject()
	require.True(t, ok)
	require.Contains(t, attrs0, "leader_api_addr")
	require.Equal(t, "http://vault-0:8200", attrs0["leader_api_addr"])

	// Check second retry_join
	attrs1, ok := retryJoins[1].GetObject()
	require.True(t, ok)
	require.Contains(t, attrs1, "leader_api_addr")
	require.Equal(t, "http://vault-1:8200", attrs1["leader_api_addr"])
}
