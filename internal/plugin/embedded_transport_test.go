package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestEmbeddedTransportMarshalRoundTrip(t *testing.T) {
	transport := newEmbeddedTransport()
	transport.SSH.Values = testMapPropertiesToStruct([]testProperty{
		{"user", "ubuntu", transport.SSH.User},
		{"host", "localhost", transport.SSH.Host},
		{"private_key", "PRIVATE KEY", transport.SSH.PrivateKey},
		{"private_key_path", "/path/to/key.pem", transport.SSH.PrivateKeyPath},
		{"passphrase", "secret", transport.SSH.Passphrase},
		{"passphrase_path", "/path/to/passphrase.txt", transport.SSH.PassphrasePath},
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
	assert.Equal(t, transport.SSH.Values, newTransport.SSH.Values)
}

func TestProviderEmbeddedTransportMergeInto(t *testing.T) {
	// Create a new provider and then make a copy from its config
	provider := newProvider()
	provider.config.Transport.SSH.Values = testMapPropertiesToStruct([]testProperty{
		{"user", "ubuntu", provider.config.Transport.SSH.User},
		{"private_key", "PRIVATE KEY", provider.config.Transport.SSH.PrivateKey},
		{"private_key_path", "/path/to/key.pem", provider.config.Transport.SSH.PrivateKeyPath},
		{"passphrase", "secret", provider.config.Transport.SSH.Passphrase},
		{"passphrase_path", "/path/to/passphrase.txt", provider.config.Transport.SSH.PassphrasePath},
	})

	provider2 := newProvider()
	require.NoError(t, provider2.config.FromTerraform5Value(provider.Config()))

	// Build a few resource transports and the expected values when merged into
	// the provider config
	resourceTransport1 := newEmbeddedTransport()
	resourceTransport1.SSH.Values = testMapPropertiesToStruct([]testProperty{
		{"host", "host1", resourceTransport1.SSH.Host},
		{"user", "admin", resourceTransport1.SSH.User},
		{"private_key", "ANOTHER PRIVATE KEY", resourceTransport1.SSH.PrivateKey},
	})
	expectedValues1 := map[string]tftypes.Value{}
	expectedValues1["host"] = resourceTransport1.SSH.Host.TFValue()
	expectedValues1["user"] = resourceTransport1.SSH.User.TFValue()
	expectedValues1["private_key"] = resourceTransport1.SSH.PrivateKey.TFValue()

	resourceTransport2 := newEmbeddedTransport()
	resourceTransport2.SSH.Values = testMapPropertiesToStruct([]testProperty{
		{"host", "host2", resourceTransport2.SSH.Host},
		{"private_key_path", "/path/to/another/key.pem", resourceTransport2.SSH.PrivateKeyPath},
	})
	expectedValues2 := map[string]tftypes.Value{}
	expectedValues2["host"] = resourceTransport2.SSH.Host.TFValue()
	expectedValues2["private_key_path"] = resourceTransport2.SSH.PrivateKeyPath.TFValue()

	resourceTransport3 := newEmbeddedTransport()
	resourceTransport3.SSH.Values = testMapPropertiesToStruct([]testProperty{
		{"host", "host3", resourceTransport3.SSH.Host},
		{"passphrase", "another secret", resourceTransport3.SSH.Passphrase},
		{"passphrase_path", "/path/to/another/passphrase.txt", resourceTransport3.SSH.PassphrasePath},
	})
	expectedValues3 := map[string]tftypes.Value{}
	expectedValues3["host"] = resourceTransport3.SSH.Host.TFValue()
	expectedValues3["passphrase"] = resourceTransport3.SSH.Passphrase.TFValue()
	expectedValues3["passphrase_path"] = resourceTransport3.SSH.PassphrasePath.TFValue()

	for _, test := range []struct {
		transport *embeddedTransportV1
		values    map[string]tftypes.Value
	}{
		{resourceTransport1, expectedValues1},
		{resourceTransport2, expectedValues2},
		{resourceTransport3, expectedValues3},
	} {
		// Get a new instance of the transport built with the provider config
		transport, err := provider2.config.Transport.Copy()
		require.NoError(t, err)

		// Set missing expected values from the provider defaults
		for k, v := range transport.SSH.Values {
			if _, ok := test.values[k]; !ok {
				test.values[k] = v
			}
		}

		// Merge our resource overrides into our transport
		test.transport.MergeInto(transport)

		// Validate that all of the overrides and the defaults from the provider are correct
		assert.Equal(t, test.values["user"], transport.SSH.User.TFValue())
		assert.Equal(t, test.values["private_key"], transport.SSH.PrivateKey.TFValue())
		assert.Equal(t, test.values["private_key_path"], transport.SSH.PrivateKeyPath.TFValue())
		assert.Equal(t, test.values["passphrase"], transport.SSH.Passphrase.TFValue())
		assert.Equal(t, test.values["passphrase_path"], transport.SSH.PassphrasePath.TFValue())
		assert.Equal(t, test.values["host"], transport.SSH.Host.TFValue())
		assert.Equal(t, test.values, transport.SSH.Values)
	}
}
