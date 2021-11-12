package consul

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigToHCL(t *testing.T) {
	expected := `datacenter = "dc1"
data_dir = "/opt/consul/data"
retry_join = ["provider=aws tag_key=Type tag_value=dc1",
]
server = true
bootstrap_expect = 3
log_file = "/var/log"
log_level = "INFO"
`

	cfg := &HCLConfig{
		Datacenter:      "dc1",
		DataDir:         "/opt/consul/data",
		RetryJoin:       []string{"provider=aws tag_key=Type tag_value=dc1"},
		Server:          true,
		BootstrapExpect: 3,
		LogFile:         "/var/log",
		LogLevel:        "INFO",
	}

	hcl, err := cfg.ToHCL()
	require.NoError(t, err)
	require.Equal(t, expected, hcl)
}
