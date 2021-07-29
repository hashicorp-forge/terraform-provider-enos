package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

var testProviders = map[string]func() (tfprotov5.ProviderServer, error){
	"enos": func() (tfprotov5.ProviderServer, error) {
		return Server(), nil
	},
}

func TestProviderSchemaMarshalRoundtrip(t *testing.T) {
	provider := newProvider()
	provider.config.Transport.SSH.Values = testMapPropertiesToStruct([]testProperty{
		{"user", "ubuntu", provider.config.Transport.SSH.User},
		{"host", "localhost", provider.config.Transport.SSH.Host},
		{"private_key", "PRIVATE KEY", provider.config.Transport.SSH.PrivateKey},
		{"private_key_path", "/path/to/key.pem", provider.config.Transport.SSH.PrivateKeyPath},
		{"passphrase", "secret", provider.config.Transport.SSH.Passphrase},
		{"passphrase_path", "/path/to/passphrase.txt", provider.config.Transport.SSH.PassphrasePath},
	})

	marshaled, err := marshal(provider.config)
	require.NoError(t, err)

	newProvider := newProvider()
	err = unmarshal(newProvider.config, marshaled)
	require.NoError(t, err)

	assert.Equal(t, provider.config.Transport.SSH.User, newProvider.config.Transport.SSH.User)
	assert.Equal(t, provider.config.Transport.SSH.Host, newProvider.config.Transport.SSH.Host)
	assert.Equal(t, provider.config.Transport.SSH.PrivateKey, newProvider.config.Transport.SSH.PrivateKey)
	assert.Equal(t, provider.config.Transport.SSH.PrivateKeyPath, newProvider.config.Transport.SSH.PrivateKeyPath)
	assert.Equal(t, provider.config.Transport.SSH.Passphrase, newProvider.config.Transport.SSH.Passphrase)
	assert.Equal(t, provider.config.Transport.SSH.PassphrasePath, newProvider.config.Transport.SSH.PassphrasePath)
}
