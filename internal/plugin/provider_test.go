package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/enos-provider/internal/diags"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/hashicorp/enos-provider/internal/server/state"
)

func TestProviderSchemaMarshalRoundtrip(t *testing.T) {
	t.Parallel()

	provider := newProvider()
	diagnosticsDir := newTfString()
	diagnosticsDir.Set("/this/is/where/the/diagnostics/be/at")
	provider.config.Transport = transportconfig{}.ssh(map[string]interface{}{
		"user":             "ubuntu",
		"host":             "localhost",
		"private_key":      "PRIVATE KEY",
		"private_key_path": "/path/to/key.pem",
		"passphrase":       "secret",
		"passphrase_path":  "/path/to/passphrase.txt",
	}).build(t)
	provider.config.DebugDataRootDir = diagnosticsDir

	marshaled, err := state.Marshal(provider.config)
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
	dir, ok := provider.config.DebugDataRootDir.Get()
	assert.True(t, ok)
	newDir, ok := newProvider.config.DebugDataRootDir.Get()
	assert.True(t, ok)
	assert.Equal(t, dir, newDir)
}

//nolint:paralleltest// because our test modifies the environment
func TestDebugDataRootDirFromEnvVar(t *testing.T) {
	debugDir := t.TempDir()

	transportCfg := transportconfig{}.ssh(sshConfig).k8s(k8sConfig).nomad(nomadConfig)
	transport := transportCfg.build(t)
	transport.Terraform5Value()

	cfg := newProviderConfig()
	cfg.Transport = transport
	val, err := state.Marshal(cfg)
	require.NoError(t, err)

	t.Setenv(enosDebugDataRootDirEnvVarKey, debugDir)

	provider := newProvider()
	resp, err := provider.Configure(context.Background(), &tfprotov6.ConfigureProviderRequest{
		TerraformVersion: "1.3",
		Config:           val,
	})
	require.NoError(t, err)
	assert.False(t, diags.HasErrors(resp.Diagnostics))

	assert.Equal(t, debugDir, provider.config.DebugDataRootDir.Val)

	requireTransportCfggg(t, provider.config.Transport, transportCfg)

	resetEnv(t)
}

//nolint:paralleltest// because our test modifies the environment
func TestDebugDataRootDirFromEnvVarOverridesProviderConfigured(t *testing.T) {
	debugDir := t.TempDir()

	transportCfg := transportconfig{}.ssh(sshConfig).k8s(k8sConfig).nomad(nomadConfig)
	transport := transportCfg.build(t)
	transport.Terraform5Value()

	cfg := newProviderConfig()
	cfg.Transport = transport
	cfg.DebugDataRootDir.Set("/this/is/where/I/thought/the/debug/should/be")
	val, err := state.Marshal(cfg)
	require.NoError(t, err)

	t.Setenv(enosDebugDataRootDirEnvVarKey, debugDir)

	provider := newProvider()
	resp, err := provider.Configure(context.Background(), &tfprotov6.ConfigureProviderRequest{
		TerraformVersion: "1.3",
		Config:           val,
	})
	require.NoError(t, err)
	assert.False(t, diags.HasErrors(resp.Diagnostics))

	assert.Equal(t, debugDir, provider.config.DebugDataRootDir.Val)

	requireTransportCfggg(t, provider.config.Transport, transportCfg)

	resetEnv(t)
}
