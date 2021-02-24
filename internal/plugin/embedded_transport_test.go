package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedTransportMarshalRoundTrip(t *testing.T) {
	transport := newEmbeddedTransport()
	transport.SSH.User = "ubuntu"
	transport.SSH.Host = "localhost"
	transport.SSH.PrivateKey = "PRIVATE KEY"
	transport.SSH.PrivateKeyPath = "/path/to/key.pem"
	transport.SSH.Passphrase = "secret"
	transport.SSH.PassphrasePath = "/path/to/pass"

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
