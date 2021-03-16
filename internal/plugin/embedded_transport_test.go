package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedTransportMarshalRoundTrip(t *testing.T) {
	transport := newEmbeddedTransport()
	transport.SSH.Values = testMapPropertiesToStruct([]testProperty{
		{"user", "ubuntu", &transport.SSH.User},
		{"host", "localhost", &transport.SSH.Host},
		{"private_key", "PRIVATE KEY", &transport.SSH.PrivateKey},
		{"private_key_path", "/path/to/key.pem", &transport.SSH.PrivateKeyPath},
		{"passphrase", "secret", &transport.SSH.Passphrase},
		{"passphrase_path", "/path/to/passphrase.txt", &transport.SSH.PassphrasePath},
	})

	marshaled, err := marshal(transport)
	require.NoError(t, err)

	newTransport := newEmbeddedTransport()
	err = unmarshal(newTransport, marshaled)
	require.NoError(t, err)

	assert.Equal(t, transport.SSH.User, newTransport.SSH.User)
	assert.Equal(t, transport.SSH.Host, newTransport.SSH.Host)
	assert.Equal(t, transport.SSH.PrivateKey, newTransport.SSH.PrivateKey)
	assert.Equal(t, transport.SSH.PrivateKeyPath, newTransport.SSH.PrivateKeyPath)
	assert.Equal(t, transport.SSH.Passphrase, newTransport.SSH.Passphrase)
	assert.Equal(t, transport.SSH.PassphrasePath, newTransport.SSH.PassphrasePath)
}

func TestProviderTransportMergeIntoResourceTransport(t *testing.T) {
	provider := newProvider()
	provider.config.Transport.SSH.Values = testMapPropertiesToStruct([]testProperty{
		{"user", "ubuntu", &provider.config.Transport.SSH.User},
		{"private_key", "PRIVATE KEY", &provider.config.Transport.SSH.PrivateKey},
		{"private_key_path", "/path/to/key.pem", &provider.config.Transport.SSH.PrivateKeyPath},
		{"passphrase", "secret", &provider.config.Transport.SSH.Passphrase},
		{"passphrase_path", "/path/to/passphrase.txt", &provider.config.Transport.SSH.PassphrasePath},
	})

	provider2 := newProvider()
	require.NoError(t, provider2.config.FromTerraform5Value(provider.Config()))

	resourceTransport := newEmbeddedTransport()
	resourceTransport.SSH.Values = testMapPropertiesToStruct([]testProperty{
		{"host", "localhost", &resourceTransport.SSH.Host},
	})

	combinedValues := provider2.config.Transport.SSH.Values
	combinedValues["host"] = tfMarshalStringValue("localhost")

	require.NoError(t, provider2.config.Transport.MergeInto(resourceTransport))

	require.Equal(t, "ubuntu", resourceTransport.SSH.User)
	require.Equal(t, "PRIVATE KEY", resourceTransport.SSH.PrivateKey)
	require.Equal(t, "/path/to/key.pem", resourceTransport.SSH.PrivateKeyPath)
	require.Equal(t, "secret", resourceTransport.SSH.Passphrase)
	require.Equal(t, "/path/to/passphrase.txt", resourceTransport.SSH.PassphrasePath)
	require.Equal(t, "localhost", resourceTransport.SSH.Host)
	require.Equal(t, combinedValues, resourceTransport.SSH.Values)
}
