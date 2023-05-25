package vault

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHAStatusDeserialize(t *testing.T) {
	t.Parallel()
	expected := NewHAStatusResponse()
	expected.Data.Nodes = []*HAStatusNode{
		{
			ActiveNode:     false,
			APIAddress:     "http://10.13.10.114:8200",
			ClusterAddress: "https://10.13.10.114:8201",
			Hostname:       "ip-10-13-10-114.us-west-1.compute.internal",
			LastEcho:       "2023-05-10T21:45:31.911061598Z",
			Version:        "1.14.0",
			RedundancyZone: "",
			UpgradeVersion: "1.14.0",
		},
		{
			ActiveNode:     false,
			APIAddress:     "http://10.13.10.150:8200",
			ClusterAddress: "https://10.13.10.150:8201",
			Hostname:       "ip-10-13-10-150.us-west-1.compute.internal",
			LastEcho:       "2023-05-10T21:45:31.79673629Z",
			Version:        "1.14.0",
			RedundancyZone: "",
			UpgradeVersion: "1.14.0",
		},
		{
			ActiveNode:     true,
			APIAddress:     "http://10.13.10.239:8200",
			ClusterAddress: "https://10.13.10.239:8201",
			Hostname:       "ip-10-13-10-239.us-west-1.compute.internal",
			LastEcho:       "",
			Version:        "1.14.0",
			RedundancyZone: "",
			UpgradeVersion: "1.14.0",
		},
	}

	got := NewHAStatusResponse()
	body := testReadSupport(t, "ha-status.json")
	require.NoError(t, json.Unmarshal(body, got))

	require.EqualValues(t, expected, got)
}
