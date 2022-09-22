package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderSchemaMarshalRoundtrip(t *testing.T) {
	provider := newProvider()
	provider.config.Transport = transportconfig{}.ssh(map[string]interface{}{
		"user":             "ubuntu",
		"host":             "localhost",
		"private_key":      "PRIVATE KEY",
		"private_key_path": "/path/to/key.pem",
		"passphrase":       "secret",
		"passphrase_path":  "/path/to/passphrase.txt",
	}).build(t)

	marshaled, err := marshal(provider.config)
	require.NoError(t, err)

	newProvider := newProvider()
	err = unmarshal(newProvider.config, marshaled)
	require.NoError(t, err)

	SSH, ok := provider.config.Transport.SSH()
	assert.True(t, ok)
	newSSH, ok := newProvider.config.Transport.SSH()
	assert.True(t, ok)
	assert.Equal(t, SSH.User, newSSH.User)
	assert.Equal(t, SSH.Host, newSSH.Host)
	assert.Equal(t, SSH.PrivateKey, newSSH.PrivateKey)
	assert.Equal(t, SSH.PrivateKeyPath, newSSH.PrivateKeyPath)
	assert.Equal(t, SSH.Passphrase, newSSH.Passphrase)
	assert.Equal(t, SSH.PassphrasePath, newSSH.PassphrasePath)
}
