package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	it "github.com/hashicorp/enos-provider/internal/transport"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	sshConfig = configmap{
		"host":             "localhost",
		"user":             "ubuntu",
		"private_key":      "PRIVATE KEY",
		"private_key_path": "/path/to/key.pem",
		"passphrase":       "secret",
		"passphrase_path":  "/path/to/passphrase.txt",
	}
	k8sConfig = configmap{
		"kubeconfig_base64": "some kubeconfig",
		"context_name":      "milky_way",
		"namespace":         "space_the_final_frontier",
		"pod":               "space_pod",
		"container":         "space_capsule",
	}
)

func TestEmbeddedTransportMarshalRoundTrip(t *testing.T) {
	transport := transportconfig{}.ssh(sshConfig).k8s(k8sConfig).build(t)

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
	assert.Equal(t, transport.K8S.KubeConfigBase64, newTransport.K8S.KubeConfigBase64)
	assert.Equal(t, transport.K8S.ContextName, newTransport.K8S.ContextName)
	assert.Equal(t, transport.K8S.Namespace, newTransport.K8S.Namespace)
	assert.Equal(t, transport.K8S.Pod, newTransport.K8S.Pod)
	assert.Equal(t, transport.K8S.Container, newTransport.K8S.Container)
}

func TestProviderEmbeddedTransportFromTFValue(t *testing.T) {
	t.Parallel()

	sshTransport := transportconfig{}.ssh(sshConfig)
	k8sTransport := transportconfig{}.k8s(k8sConfig)
	sshAndK8STransport := transportconfig{}.ssh(sshConfig).k8s(k8sConfig)
	noTransport := transportconfig{}

	partialSSHTransport := transportconfig{}.ssh(configmap{
		"host": "10.0.5.6",
		"user": "admin",
	})
	partialSSHConfiguredExpected := transportconfig{}.ssh(configmap{
		"host": "10.0.5.6",
		"user": "admin",
	})
	partialSSHAttributesExpected := transportconfig{}.ssh(configmap{
		"host":             "10.0.5.6",
		"user":             "admin",
		"private_key":      nil,
		"private_key_path": nil,
		"passphrase":       nil,
		"passphrase_path":  nil,
	})
	partialK8STransport := transportconfig{}.
		k8sValue("kubeconfig_base64", "some kubeconfig").
		k8sValue("pod", "space_pod")

	partialK8SConfiguredExpected := transportconfig{}.
		k8sValue("kubeconfig_base64", "some kubeconfig").
		k8sValue("pod", "space_pod")
	partialK8SAttributesExpected := transportconfig{}.
		k8sValue("kubeconfig_base64", "some kubeconfig").
		k8sValue("context_name", nil).
		k8sValue("namespace", nil).
		k8sValue("pod", "space_pod").
		k8sValue("container", nil)

	for _, test := range []struct {
		name                    string
		config                  transportconfig
		expectedAttributeValues transportconfig
		expectedValues          transportconfig
	}{
		{"only_ssh_configured", sshTransport, sshTransport, sshTransport},
		{"only_k8s_configured", k8sTransport, k8sTransport, k8sTransport},
		{"ssh_and_k8s_configured", sshAndK8STransport, sshAndK8STransport, sshAndK8STransport},
		{"none_configured", noTransport, noTransport, noTransport},
		{"partial_ssh_configured", partialSSHTransport, partialSSHAttributesExpected, partialSSHConfiguredExpected},
		{"partial_k8s_configured", partialK8STransport, partialK8SAttributesExpected, partialK8SConfiguredExpected},
	} {
		t.Run(test.name, func(tt *testing.T) {
			expectedAttributeValues := test.expectedAttributeValues
			expectedValues := test.expectedValues

			transport := newEmbeddedTransport()
			require.NoError(t, transport.FromTerraform5Value(test.config.toTFValue(t)))
			assert.Equal(tt, len(expectedValues), len(transport.CopyValues()))
			assert.Equal(tt, len(expectedAttributeValues), len(transport.Attributes()))

			for transportType, actualValues := range transport.CopyValues() {
				expectedValues := expectedValues[transportType]
				assert.Equal(tt, len(expectedValues), len(actualValues))
				for name, expectedValue := range expectedValues {
					actualStringValue := ""
					actualValue, ok := actualValues[name]
					if ok {
						require.NoError(tt, actualValue.As(&actualStringValue))
					}
					assert.Equal(tt, expectedValue, actualStringValue)
				}
			}

			for transportType, actualValues := range transport.Attributes() {
				expectedTransportValues := expectedAttributeValues[transportType]
				assert.Equal(tt, len(expectedTransportValues), len(actualValues))
				for name, expectedValue := range expectedTransportValues {
					actualValue, ok := actualValues[name].(*tfString).Get()
					if ok {
						assert.Equal(tt, expectedValue, actualValue)
					} else {
						assert.Equal(tt, "", actualValue)
					}
				}
			}
		})
	}
}

func TestProviderEmbeddedTransportApplyDefaults(t *testing.T) {
	t.Parallel()
	// Create a new provider and then make a copy from its config
	provider := newProvider()
	provider.config.Transport = transportconfig{}.ssh(sshConfig).k8s(k8sConfig).build(t)
	defaultsTransport := provider.config.Transport

	// Build a few resource transports and the expected values when merged into
	// the provider config
	config1 := transportconfig{}.ssh(configmap{
		"host":        "host1",
		"user":        "admin",
		"private_key": "ANOTHER PRIVATE KEY",
	})
	expectedValues1 := transportconfig{}.ssh(sshConfig).
		sshValue("host", "host1").
		sshValue("user", "admin").
		sshValue("private_key", "ANOTHER PRIVATE KEY")

	config2 := transportconfig{}.ssh(configmap{
		"host":             "host2",
		"private_key_path": "/path/to/another/key.pem",
	})
	expectedValues2 := transportconfig{}.ssh(sshConfig).
		sshValue("host", "host2").
		sshValue("private_key_path", "/path/to/another/key.pem")

	config3 := transportconfig{}.ssh(configmap{
		"host":            "host3",
		"passphrase":      "another secret",
		"passphrase_path": "/path/to/another/passphrase.txt",
	})
	expectedValues3 := transportconfig{}.ssh(sshConfig).
		sshValue("host", "host3").
		sshValue("passphrase", "another secret").
		sshValue("passphrase_path", "/path/to/another/passphrase.txt")

	configK8S := transportconfig{}.k8s(configmap{
		"kubeconfig_base64": "some kubeconfig",
		"context_name":      "some-context",
	})
	expectedValuesK8S := transportconfig{}.k8s(k8sConfig).
		k8sValue("kubeconfig_base64", "some kubeconfig").
		k8sValue("context_name", "some-context")

	for _, test := range []struct {
		name              string
		config            transportconfig
		defaultsTransport *embeddedTransportV1
		expectedValues    transportconfig
	}{
		{"transport_ssh_1", config1, defaultsTransport, expectedValues1},
		{"transport_ssh_2", config2, defaultsTransport, expectedValues2},
		{"transport_ssh_3", config3, defaultsTransport, expectedValues3},
		{"transport_k8s", configK8S, defaultsTransport, expectedValuesK8S},
	} {
		t.Run(test.name, func(tt *testing.T) {
			// Apply our provider defaults to our transport
			transport := test.config.build(tt)
			require.NoError(tt, transport.ApplyDefaults(test.defaultsTransport))

			for transportName, expectedConfig := range test.expectedValues {
				actualConfig := transport.Attributes()[transportName]
				assert.Equal(tt, len(actualConfig), len(expectedConfig))
				for name, expectedValue := range expectedConfig {
					assert.Equal(tt, expectedValue, actualConfig[name].(*tfString).Val)
				}
			}
		})
	}
}

func TestProviderEmbeddedTransportValidate(t *testing.T) {
	t.Parallel()

	config1 := transportconfig{}.ssh(sshConfig).k8s(k8sConfig)

	config2 := transportconfig{}

	config3 := transportconfig{}.ssh(configmap{
		"user":             "ubuntu",
		"host":             "localhost",
		"private_key":      "PRIVATE KEY",
		"private_key_path": "/path/to/key.pem",
		"passphrase":       "secret",
		"passphrase_path":  "/path/to/passphrase.txt",
	})

	config4 := transportconfig{}.k8s(configmap{
		"kubeconfig_base64": "some kubeconfig",
		"context_name":      "mexican-food",
		"namespace":         "tacos",
		"pod":               "hard-shell",
		"container":         "cheese",
	})

	for _, test := range []struct {
		name    string
		config  transportconfig
		wantErr bool
	}{
		{"invalid_both_configured", config1, true},
		{"invalid_none_configured", config2, true},
		{"valid_ssh_configured", config3, false},
		{"valid_kubernetes_configured", config4, false},
	} {
		t.Run(test.name, func(tt *testing.T) {
			transport := test.config.build(tt)
			if test.wantErr {
				require.Error(tt, transport.Validate(context.Background()))
			} else {
				require.NoError(tt, transport.Validate(context.Background()))
			}
		})
	}
}

func TestProviderEmbeddedTransportCopy(t *testing.T) {
	t.Parallel()

	sshAndK8sConfig := transportconfig{}.ssh(sshConfig).k8s(k8sConfig)
	sshOnlyConfig := transportconfig{}.ssh(sshConfig)
	k8sOnlyConfig := transportconfig{}.k8s(k8sConfig)

	for _, test := range []struct {
		name   string
		config transportconfig
	}{
		{"ssh_and_k8s_config", sshAndK8sConfig},
		{"only_ssh_config", sshOnlyConfig},
		{"only_k8s_config", k8sOnlyConfig},
	} {
		t.Run(test.name, func(tt *testing.T) {
			transport := test.config.build(tt)
			transportCopy, err := transport.Copy()
			require.NoError(tt, err)

			transportCopy.SSH.Host.Set("copy-host")
			transportCopy.SSH.User.Set("copy-user")
			transportCopy.SSH.PrivateKey.Set("copy-private-key")
			transportCopy.SSH.PrivateKeyPath.Set("copy-private-key-path")
			transportCopy.SSH.Passphrase.Set("copy-passphrase")
			transportCopy.SSH.PassphrasePath.Set("copy-passphrase-path")
			transportCopy.K8S.KubeConfigBase64.Set("copy-kubeconfig-path")
			transportCopy.K8S.ContextName.Set("copy-context-name")
			transportCopy.K8S.Namespace.Set("copy-namespace")
			transportCopy.K8S.Pod.Set("copy-pod")
			transportCopy.K8S.Container.Set("copy-container")

			for tType, tConfig := range test.config {
				if len(tConfig) > 0 {
					for attr, value := range tConfig {
						switch tType {
						case "ssh":
							switch attr {
							case "host":
								assert.Equal(tt, value, transport.SSH.Host.Val)
							case "user":
								assert.Equal(tt, value, transport.SSH.User.Val)
							case "private-key":
								assert.Equal(tt, value, transport.SSH.PrivateKey.Val)
							case "private-key-path":
								assert.Equal(tt, value, transport.SSH.PrivateKeyPath.Val)
							case "passphrase":
								assert.Equal(tt, value, transport.SSH.Passphrase.Val)
							case "passphrase-path":
								assert.Equal(tt, value, transport.SSH.PassphrasePath.Val)
							}
						case "k8s":
							switch attr {
							case "kubeconfig-path":
								assert.Equal(tt, value, transport.K8S.KubeConfigBase64.Val)
							case "context-name":
								assert.Equal(tt, value, transport.K8S.ContextName.Val)
							case "namespace":
								assert.Equal(tt, value, transport.K8S.Namespace.Val)
							case "pod":
								assert.Equal(tt, value, transport.K8S.Pod.Val)
							case "container":
								assert.Equal(tt, value, transport.K8S.Container.Val)
							}
						}
					}
				}
			}
		})
	}
}

func TestProviderEmbeddedTransportClient(t *testing.T) {
	t.Parallel()

	sshAndK8sConfig := transportconfig{}.ssh(sshConfig).k8s(k8sConfig)
	sshOnlyConfig := transportconfig{}.ssh(sshConfig)
	k8sOnlyConfig := transportconfig{}.k8s(k8sConfig)
	noTransportConfig := transportconfig{}

	for _, test := range []struct {
		name          string
		config        transportconfig
		wantErr       bool
		wantSSHClient bool
		wantK8SClient bool
	}{
		{"ssh_and_k8s_config", sshAndK8sConfig, true, false, false},
		{"only_ssh_config", sshOnlyConfig, false, true, false},
		{"only_k8s_config", k8sOnlyConfig, false, false, true},
		{"no_configured_transport", noTransportConfig, true, false, false},
	} {
		t.Run(test.name, func(tt *testing.T) {
			transport := test.config.build(tt)
			transport.SSH.sshTransportBuilder = func(state *embeddedTransportSSHv1, ctx context.Context) (it.Transport, error) {
				if !test.wantSSHClient {
					t.Error("An ssh client should not have been created but was")
				}
				return nil, nil
			}
			transport.K8S.k8sTransportBuilder = func(state *embeddedTransportK8Sv1, ctx context.Context) (it.Transport, error) {
				if !test.wantK8SClient {
					t.Error("A k8s client should not have been created but was")
				}
				return nil, nil
			}

			if test.wantErr {
				_, err := transport.Client(context.Background())
				require.Error(tt, err)
			} else {
				_, err := transport.Client(context.Background())
				require.NoError(tt, err)
			}
		})
	}
}

// helpers
type transportconfig map[string]configmap

func (tc transportconfig) k8s(config configmap) transportconfig {
	tc["kubernetes"] = config.copy()
	return tc
}

func (tc transportconfig) k8sValue(key string, value interface{}) transportconfig {
	if _, ok := tc["kubernetes"]; !ok {
		tc["kubernetes"] = configmap{}
	}
	tc["kubernetes"][key] = value
	return tc
}

func (tc transportconfig) ssh(config configmap) transportconfig {
	tc["ssh"] = config.copy()
	return tc
}

func (tc transportconfig) sshValue(key string, value interface{}) transportconfig {
	if _, ok := tc["ssh"]; !ok {
		tc["ssh"] = configmap{}
	}
	tc["ssh"][key] = value
	return tc
}

func (tc transportconfig) build(t *testing.T) *embeddedTransportV1 {
	t.Helper()
	transport := newEmbeddedTransport()
	require.NoError(t, transport.FromTerraform5Value(tc.toTFValue(t)))
	return transport
}

type configmap map[string]interface{}

func (c configmap) copy() configmap {
	newConfigmap := configmap{}
	for k, v := range c {
		newConfigmap[k] = v
	}
	return newConfigmap
}

func (tc transportconfig) toTFValue(t *testing.T) tftypes.Value {
	t.Helper()

	values := map[string]tftypes.Value{}
	types := map[string]tftypes.Type{}

	for transportType, cfg := range tc {
		types[transportType] = tftypes.Map{ElementType: tftypes.String}
		transportValues := map[string]tftypes.Value{}
		for name, val := range cfg {
			transportValues[name] = tftypes.NewValue(tftypes.String, val)
		}
		values[transportType] = tftypes.NewValue(types[transportType], transportValues)
	}

	return tftypes.NewValue(tftypes.Object{AttributeTypes: types}, values)
}
