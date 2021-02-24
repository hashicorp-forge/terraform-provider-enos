package plugin

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourceTransport(t *testing.T) {
	cfg := `data "enos_transport" "foo" {
	ssh {
		user = "ubuntu"
		host = "hostname"
		private_key = "BEGIN PRIVATE KEY"
		private_key_path = "/path/to/key.pem"
		passphrase = "secret"
		passphrase_path = "/path/to/passphrase"
	}
}`

	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testProviders,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("data.enos_transport.foo", "ssh.user", regexp.MustCompile(`^ubuntu$`)),
					resource.TestMatchResourceAttr("data.enos_transport.foo", "ssh.host", regexp.MustCompile(`^hostname$`)),
					resource.TestMatchResourceAttr("data.enos_transport.foo", "ssh.private_key", regexp.MustCompile(`^BEGIN PRIVATE KEY$`)),
					resource.TestMatchResourceAttr("data.enos_transport.foo", "ssh.private_key_path", regexp.MustCompile(`^/path/to/key.pem$`)),
					resource.TestMatchResourceAttr("data.enos_transport.foo", "ssh.passphrase", regexp.MustCompile(`^secret$`)),
					resource.TestMatchResourceAttr("data.enos_transport.foo", "ssh.passphrase_path", regexp.MustCompile(`^/path/to/passphrase$`)),
					resource.TestMatchResourceAttr("data.enos_transport.foo", "out.ssh.user", regexp.MustCompile(`^ubuntu$`)),
					resource.TestMatchResourceAttr("data.enos_transport.foo", "out.ssh.host", regexp.MustCompile(`^hostname$`)),
					resource.TestMatchResourceAttr("data.enos_transport.foo", "out.ssh.private_key", regexp.MustCompile(`^BEGIN PRIVATE KEY$`)),
					resource.TestMatchResourceAttr("data.enos_transport.foo", "out.ssh.private_key_path", regexp.MustCompile(`^/path/to/key.pem$`)),
					resource.TestMatchResourceAttr("data.enos_transport.foo", "out.ssh.passphrase", regexp.MustCompile(`^secret$`)),
					resource.TestMatchResourceAttr("data.enos_transport.foo", "out.ssh.passphrase_path", regexp.MustCompile(`^/path/to/passphrase$`)),
				),
			},
		},
	})
}

func TestDataSourceTransportMarshalRoundtrip(t *testing.T) {
	state := newDataTransportState()
	state.Transport.SSH.User = "ubuntu"
	state.Transport.SSH.Host = "localhost"
	state.Transport.SSH.PrivateKey = "PRIVATE KEY"
	state.Transport.SSH.PrivateKeyPath = "/path/to/key.pem"
	state.Transport.SSH.Passphrase = "secret"
	state.Transport.SSH.PassphrasePath = "/path/to/passprhase"

	marshaled, err := marshal(state)
	require.NoError(t, err)

	newState := newDataTransportState()
	err = unmarshal(newState, marshaled)
	require.NoError(t, err)

	assert.Equal(t, state.Transport.SSH.User, newState.Transport.SSH.User)
	assert.Equal(t, state.Transport.SSH.Host, newState.Transport.SSH.Host)
	assert.Equal(t, state.Transport.SSH.PrivateKey, newState.Transport.SSH.PrivateKey)
	assert.Equal(t, state.Transport.SSH.PrivateKeyPath, newState.Transport.SSH.PrivateKeyPath)
	assert.Equal(t, state.Transport.SSH.Passphrase, newState.Transport.SSH.Passphrase)
	assert.Equal(t, state.Transport.SSH.PassphrasePath, newState.Transport.SSH.PassphrasePath)
}
