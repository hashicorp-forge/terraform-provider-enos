package vault

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigToHCL(t *testing.T) {
	expected := `api_addr = "https://192.0.0.1:8200"
cache_size = "1234"
cluster_addr = "https://192.0.0.1:8201"
cluster_name = "cluster"
default_max_request_duration = "60s"
default_lease_ttl = "20h"
disable_cache = true
disable_clustering = true
disable_mlock = true
disable_performance_standby = true
disable_sealwrap = true
ha_storage "raft" {
  node_id = "vault_1"
  path = "$demo_home/ha-raft_1"
}
listener "tcp" {
  address = "127.0.0.1:8200"
  tls_disable = 1
}
log_level = "debug"
max_lease_ttl = "40h"
pid_file = "/var/run/vault.pid"
PluginDirectory = "/opt/vault/plugins"
raw_storage_endpoint = true
seal "awskms" {
  kms_key_id = "my-key-id"
}
storage "consul" {
  address = "127.0.0.1:8500"
  path = "vault"
}
telemetry {
  disable_hostname = true
  statsite_address = "127.0.0.1:8125"
}
ui = true
`

	cfg := &HCLConfig{
		APIAddr:                   "https://192.0.0.1:8200",
		CacheSize:                 "1234",
		ClusterAddr:               "https://192.0.0.1:8201",
		ClusterName:               "cluster",
		DefaultMaxRequestDuration: "60s",
		DefaultLeaseTTL:           "20h",
		DisableCache:              true,
		DisableClustering:         true,
		DisableMlock:              true,
		DisablePerformanceStandby: true,
		DisableSealwrap:           true,
		Listener: &HCLBlock{
			Label: "tcp",
			Attrs: HCLBlockAttrs{
				"address":     "127.0.0.1:8200",
				"tls_disable": 1,
			},
		},
		LogLevel: "debug",
		HAStorage: &HCLBlock{
			Label: "raft",
			Attrs: HCLBlockAttrs{
				"path":    "$demo_home/ha-raft_1",
				"node_id": "vault_1",
			},
		},
		MaxLeaseTTL:        "40h",
		PidFile:            "/var/run/vault.pid",
		PluginDirectory:    "/opt/vault/plugins",
		RawStorageEndpoint: true,
		Seal: &HCLBlock{
			Label: "awskms",
			Attrs: HCLBlockAttrs{
				"kms_key_id": "my-key-id",
			},
		},
		Storage: &HCLBlock{
			Label: "consul",
			Attrs: HCLBlockAttrs{
				"address": "127.0.0.1:8500",
				"path":    "vault",
			},
		},
		Telemetry: &HCLBlock{
			Attrs: HCLBlockAttrs{
				"statsite_address": "127.0.0.1:8125",
				"disable_hostname": true,
			},
		},
		UI: true,
	}

	hcl, err := cfg.ToHCL()
	require.NoError(t, err)
	require.Equal(t, expected, hcl)
}
