package vault

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHostInfoDeserialize(t *testing.T) {
	t.Parallel()
	expected := NewHostInfoResponse()
	expected.Data.Host.BootTime = "1683754328"
	expected.Data.Host.HostID = "0315c1dd-5d8b-4dc5-b960-678a0e7de257"
	expected.Data.Host.Hostname = "ip-10-13-10-150.us-west-1.compute.internal"
	expected.Data.Host.KernelArch = "x86_64"
	expected.Data.Host.KernelVersion = "4.18.0-193.105.1.el8_2.x86_64"
	expected.Data.Host.OS = "linux"
	expected.Data.Host.Platform = "redhat"
	expected.Data.Host.PlatformFamily = "rhel"
	expected.Data.Host.PlatformVersion = "8.2"
	expected.Data.Host.Procs = "94"
	expected.Data.Host.Uptime = "716"
	expected.Data.Host.VirtualizationRole = "guest"
	expected.Data.Host.VirtualizationSystem = "xen"

	got := NewHostInfoResponse()
	body := testReadSupport(t, "host-info.json")
	require.NoError(t, json.Unmarshal(body, got))
	require.EqualValues(t, expected, got)
}
