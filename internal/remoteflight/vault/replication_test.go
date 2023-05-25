package vault

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplicationStatusDeserialize(t *testing.T) {
	t.Parallel()
	expected := NewReplicationResponse()
	expected.Data.DR.ClusterID = "e4bfa800-002e-7b6d-14c2-617855ece02f"
	expected.Data.DR.KnownSecondaries = []string{"4"}
	expected.Data.DR.LastWAL = "455"
	expected.Data.DR.MerkleRoot = "cdcf796619240ce19dd8af30fa700f64c8006e3d"
	expected.Data.DR.Mode = "primary"
	expected.Data.DR.Secondaries = []*ReplicationSecondary{
		{
			APIAddress:       "https://127.0.0.1:49277",
			ClusterAddress:   "https://127.0.0.1:49281",
			ConnectionStatus: "connected",
			LastHeartbeat:    "2020-06-10T15:40:46-07:00",
			NodeID:           "4",
		},
	}
	expected.Data.Performance.ClusterID = "1598d434-dfec-1f48-f019-3d22a8075bf9"
	expected.Data.Performance.KnownSecondaries = nil
	expected.Data.Performance.MerkleRoot = "43f40fc775b40cc76cd5d7e289b2e6eaf4ba138c"
	expected.Data.Performance.Mode = "secondary"

	got := NewReplicationResponse()
	body := testReadSupport(t, "replication-status.json")
	require.NoError(t, json.Unmarshal(body, got))
	require.EqualValues(t, expected, got)
}
