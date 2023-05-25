package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/enos-provider/internal/server/state"

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
	nomadConfig = configmap{
		"host":          "http://127.0.0.1:4646",
		"secret_id":     "ILoveBananas",
		"allocation_id": "efd67cdd",
		"task_name":     "apples",
	}
)

func TestEmbeddedTransportMarshalRoundTrip(t *testing.T) {
	t.Parallel()

	transport := transportconfig{}.ssh(sshConfig).k8s(k8sConfig).nomad(nomadConfig).build(t)

	marshaled, err := state.Marshal(transport)
	require.NoError(t, err)

	newTransport := newEmbeddedTransport()
	err = unmarshal(newTransport, marshaled)
	require.NoError(t, err)

	assert.Equal(t, transport.Attributes(), newTransport.Attributes())
	assert.Equal(t, transport.CopyValues(), newTransport.CopyValues())

	ssh, ok := newTransport.SSH()
	assert.True(t, ok)
	assert.Equal(t, sshConfig["user"], ssh.User.Value())
	assert.Equal(t, sshConfig["host"], ssh.Host.Value())
	assert.Equal(t, sshConfig["private_key"], ssh.PrivateKey.Value())
	assert.Equal(t, sshConfig["private_key_path"], ssh.PrivateKeyPath.Value())
	assert.Equal(t, sshConfig["passphrase"], ssh.Passphrase.Value())
	assert.Equal(t, sshConfig["passphrase_path"], ssh.PassphrasePath.Value())
	k8s, ok := newTransport.K8S()
	assert.True(t, ok)
	assert.Equal(t, k8sConfig["kubeconfig_base64"], k8s.KubeConfigBase64.Value())
	assert.Equal(t, k8sConfig["context_name"], k8s.ContextName.Value())
	assert.Equal(t, k8sConfig["namespace"], k8s.Namespace.Value())
	assert.Equal(t, k8sConfig["pod"], k8s.Pod.Value())
	assert.Equal(t, k8sConfig["container"], k8s.Container.Value())
	nomad, ok := newTransport.Nomad()
	assert.True(t, ok)
	assert.Equal(t, nomadConfig["host"], nomad.Host.Value())
	assert.Equal(t, nomadConfig["secret_id"], nomad.SecretID.Value())
	assert.Equal(t, nomadConfig["allocation_id"], nomad.AllocationID.Value())
	assert.Equal(t, nomadConfig["task_name"], nomad.TaskName.Value())
}

func TestProviderEmbeddedTransportFromTFValue(t *testing.T) {
	t.Parallel()

	sshTransport := transportconfig{}.ssh(sshConfig)
	k8sTransport := transportconfig{}.k8s(k8sConfig)
	sshAndK8STransport := transportconfig{}.ssh(sshConfig).k8s(k8sConfig)
	allTransports := transportconfig{}.ssh(sshConfig).k8s(k8sConfig).nomad(nomadConfig)
	sshAndNomad := transportconfig{}.ssh(sshConfig).nomad(nomadConfig)
	nomadAndK8S := transportconfig{}.nomad(nomadConfig).k8s(k8sConfig)
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

	partialNomadTransport := transportconfig{}.
		nomadValue("host", "http://127.0.0.1:4646").
		nomadValue("allocation_id", "ddf76bc4")

	partialNomadConfiguredExpected := transportconfig{}.
		nomadValue("host", "http://127.0.0.1:4646").
		nomadValue("allocation_id", "ddf76bc4")
	partialNomadAttributesExpected := transportconfig{}.
		nomadValue("host", "http://127.0.0.1:4646").
		nomadValue("secret_id", nil).
		nomadValue("allocation_id", "ddf76bc4").
		nomadValue("task_name", nil)

	//nolint:paralleltest
	for _, test := range []struct {
		name                    string
		config                  transportconfig
		expectedAttributeValues transportconfig
		expectedValues          transportconfig
	}{
		{"only_ssh_configured", sshTransport, sshTransport, sshTransport},
		{"only_k8s_configured", k8sTransport, k8sTransport, k8sTransport},
		{"ssh_and_k8s_configured", sshAndK8STransport, sshAndK8STransport, sshAndK8STransport},
		{"all_transports_configured", allTransports, allTransports, allTransports},
		{"ssh_and_nomad_configured", sshAndNomad, sshAndNomad, sshAndNomad},
		{"nomad_and_k8s_configured", nomadAndK8S, nomadAndK8S, nomadAndK8S},
		{"none_configured", noTransport, noTransport, noTransport},
		{"partial_ssh_configured", partialSSHTransport, partialSSHAttributesExpected, partialSSHConfiguredExpected},
		{"partial_k8s_configured", partialK8STransport, partialK8SAttributesExpected, partialK8SConfiguredExpected},
		{"partial_nomad_configured", partialNomadTransport, partialNomadAttributesExpected, partialNomadConfiguredExpected},
	} {
		test := test
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
	defaultsTransport := transportconfig{}.ssh(sshConfig).k8s(k8sConfig).nomad(nomadConfig).build(t)
	emptyDefaultsTransport := transportconfig{}.build(t)

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

	configNomad := transportconfig{}.nomad(configmap{
		"host":          "http://10.0.4.5:4646",
		"allocation_id": "eed789gf",
	})
	expectedValuesNomad := transportconfig{}.nomad(nomadConfig).
		nomadValue("host", "http://10.0.4.5:4646").
		nomadValue("allocation_id", "eed789gf")

	configNoTransport := transportconfig{}

	configMultipleTransports := transportconfig{}.
		sshValue("host", "20.6.30.2").
		nomadValue("host", "http://127.0.0.1:4646")

	//nolint:paralleltest
	for _, test := range []struct {
		name              string
		config            transportconfig
		defaultsTransport *embeddedTransportV1
		expectedValues    transportconfig
		wantErr           bool
	}{
		{"transport_ssh_1", config1, defaultsTransport, expectedValues1, false},
		{"transport_ssh_2", config2, defaultsTransport, expectedValues2, false},
		{"transport_ssh_3", config3, defaultsTransport, expectedValues3, false},
		{"transport_k8s", configK8S, defaultsTransport, expectedValuesK8S, false},
		{"transport_nomad", configNomad, defaultsTransport, expectedValuesNomad, false},
		{"transport_nomad", configNomad, defaultsTransport, expectedValuesNomad, false},
		{"no_transport", configNoTransport, defaultsTransport, configNoTransport, true},
		{"multiple_transports", configMultipleTransports, defaultsTransport, transportconfig{}, true},
		{"no_defaults_multiple_transports", configMultipleTransports, emptyDefaultsTransport, transportconfig{}, true},
		{"no_defaults_no_transport", configNoTransport, emptyDefaultsTransport, transportconfig{}, true},
	} {
		test := test
		t.Run(test.name, func(tt *testing.T) {
			// Apply our provider defaults to our transport
			transport := test.config.build(tt)
			if test.wantErr {
				_, err := transport.ApplyDefaults(test.defaultsTransport)
				require.Error(tt, err)
			} else {
				_, err := transport.ApplyDefaults(test.defaultsTransport)
				require.NoError(tt, err)

				for tType, expectedConfig := range test.expectedValues {
					actualConfig := transport.Attributes()[tType]
					assert.Equal(tt, len(actualConfig), len(expectedConfig))
					for name, expectedValue := range expectedConfig {
						assert.Equal(tt, expectedValue, actualConfig[name].(*tfString).Val)
					}
				}
			}
		})
	}
}

func TestProviderEmbeddedTransportValidate(t *testing.T) {
	t.Parallel()

	sshAndK8s := transportconfig{}.ssh(sshConfig).k8s(k8sConfig)

	sshK8sAndNomad := transportconfig{}.ssh(sshConfig).k8s(k8sConfig).nomad(nomadConfig)

	sshAndNomad := transportconfig{}.ssh(sshConfig).nomad(nomadConfig)

	k8sAndNomad := transportconfig{}.k8s(k8sConfig).nomad(nomadConfig)

	none := transportconfig{}

	validSSH := transportconfig{}.ssh(configmap{
		"user":             "ubuntu",
		"host":             "localhost",
		"private_key":      "PRIVATE KEY",
		"private_key_path": "/path/to/key.pem",
		"passphrase":       "secret",
		"passphrase_path":  "/path/to/passphrase.txt",
	})

	validK8S := transportconfig{}.k8s(configmap{
		"kubeconfig_base64": "some kubeconfig",
		"context_name":      "mexican-food",
		"namespace":         "tacos",
		"pod":               "hard-shell",
		"container":         "cheese",
	})

	validNomad := transportconfig{}.nomad(configmap{
		"host":          "http://127.0.0.1:4646",
		"allocation_id": "df67f8c9",
		"task_name":     "some_task",
	})

	invalidSSH := transportconfig{}.ssh(configmap{
		"user": "ubuntu",
	})

	invalidK8S := transportconfig{}.k8s(configmap{
		"kubeconfig_base64": "some kubeconfig",
	})

	invalidNomad := transportconfig{}.nomad(configmap{
		"host": "http://127.0.0.1:4646",
	})

	//nolint:paralleltest// build() handles it
	for _, test := range []struct {
		name    string
		config  transportconfig
		wantErr bool
	}{
		{"invalid_ssh_and_k8s_configured", sshAndK8s, true},
		{"invalid_ssh_k8s_and_nomad_configured", sshK8sAndNomad, true},
		{"invalid_ssh_and_nomad_configured", sshAndNomad, true},
		{"invalid_k8s_and_nomad_configured", k8sAndNomad, true},
		{"invalid_none_configured", none, true},
		{"valid_ssh_configured", validSSH, false},
		{"valid_kubernetes_configured", validK8S, false},
		{"valid_nomad_configured", validNomad, false},
		{"invalid_ssh_configured", invalidSSH, true},
		{"invalid_k8s_configured", invalidK8S, true},
		{"invalid_nomad_configured", invalidNomad, true},
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

	sshAndK8SConfig := transportconfig{}.ssh(sshConfig).k8s(k8sConfig)
	sshK8SAndNomadConfig := transportconfig{}.ssh(sshConfig).k8s(k8sConfig).nomad(nomadConfig)
	k8SAndNomadConfig := transportconfig{}.k8s(k8sConfig).nomad(nomadConfig)
	sshAndNomadConfig := transportconfig{}.ssh(sshConfig).nomad(nomadConfig)
	sshOnlyConfig := transportconfig{}.ssh(sshConfig)
	k8sOnlyConfig := transportconfig{}.k8s(k8sConfig)
	nomadOnlyConfig := transportconfig{}.nomad(nomadConfig)

	//nolint:paralleltest// because build() handles it
	for _, test := range []struct {
		name   string
		config transportconfig
	}{
		{"ssh_and_k8s_config", sshAndK8SConfig},
		{"ssh_k8s_and_nomad_config", sshK8SAndNomadConfig},
		{"k8s_and_nomad_config", k8SAndNomadConfig},
		{"ssh_and_nomad_config", sshAndNomadConfig},
		{"only_ssh_config", sshOnlyConfig},
		{"only_k8s_config", k8sOnlyConfig},
		{"only_nomad_config", nomadOnlyConfig},
	} {
		t.Run(test.name, func(tt *testing.T) {
			transport := test.config.build(tt)
			transportCopy, err := transport.Copy()
			require.NoError(tt, err)

			// here we set new values to all the transport config, in order to assert that changine
			// a value in the copy does not affect the original
			ssh := newEmbeddedTransportSSH()
			ssh.Host.Set("copy-host")
			ssh.User.Set("copy-user")
			ssh.PrivateKey.Set("copy-private-key")
			ssh.PrivateKeyPath.Set("copy-private-key-path")
			ssh.Passphrase.Set("copy-passphrase")
			ssh.PassphrasePath.Set("copy-passphrase-path")
			assert.NoError(tt, transportCopy.SetTransportState(ssh))
			k8s := newEmbeddedTransportK8Sv1()
			k8s.KubeConfigBase64.Set("copy-kubeconfig-path")
			k8s.ContextName.Set("copy-context-name")
			k8s.Namespace.Set("copy-namespace")
			k8s.Pod.Set("copy-pod")
			k8s.Container.Set("copy-container")
			assert.NoError(tt, transportCopy.SetTransportState(k8s))
			nomad := newEmbeddedTransportNomadv1()
			nomad.Host.Set("foo")
			nomad.SecretID.Set("bogus")
			nomad.AllocationID.Set("oppopoop")
			nomad.TaskName.Set("not a task")
			assert.NoError(tt, transportCopy.SetTransportState(nomad))

			assertTransportCfg(tt, transport, test.config)
		})
	}
}

func TestProviderEmbeddedTransportClient(t *testing.T) {
	t.Parallel()

	sshAndK8sConfig := transportconfig{}.ssh(sshConfig).k8s(k8sConfig)
	sshAndNomadConfig := transportconfig{}.ssh(sshConfig).nomad(nomadConfig)
	k8sAndNomadConfig := transportconfig{}.k8s(k8sConfig).nomad(nomadConfig)
	sshK8SAndNomadConfig := transportconfig{}.ssh(sshConfig).k8s(k8sConfig).nomad(nomadConfig)
	sshOnlyConfig := transportconfig{}.ssh(sshConfig)
	k8sOnlyConfig := transportconfig{}.k8s(k8sConfig)
	nomadOnlyConfig := transportconfig{}.nomad(nomadConfig)
	noTransportConfig := transportconfig{}
	//nolint:paralleltest// build() handles it
	for _, test := range []struct {
		name            string
		config          transportconfig
		wantErr         bool
		wantSSHClient   bool
		wantK8SClient   bool
		wantNomadClient bool
	}{
		{"ssh_and_k8s_config", sshAndK8sConfig, true, false, false, false},
		{"ssh_k8s_and_nomad_config", sshK8SAndNomadConfig, true, false, false, false},
		{"ssh_and_nomad_config", sshAndNomadConfig, true, false, false, false},
		{"k8s_and_nomad_config", k8sAndNomadConfig, true, false, false, false},
		{"only_ssh_config", sshOnlyConfig, false, true, false, false},
		{"only_k8s_config", k8sOnlyConfig, false, false, true, false},
		{"only_nomad_config", nomadOnlyConfig, false, false, true, true},
		{"no_configured_transport", noTransportConfig, true, false, false, false},
	} {
		test := test
		t.Run(test.name, func(tt *testing.T) {
			transport := test.config.build(tt)
			if ssh, ok := transport.SSH(); ok {
				ssh.sshTransportBuilder = func(state *embeddedTransportSSHv1, ctx context.Context) (it.Transport, error) {
					if !test.wantSSHClient {
						t.Error("An ssh client should not have been created but was")
					}

					return nil, nil
				}
			}
			if k8s, ok := transport.K8S(); ok {
				k8s.k8sTransportBuilder = func(state *embeddedTransportK8Sv1, ctx context.Context) (it.Transport, error) {
					if !test.wantK8SClient {
						t.Error("A k8s client should not have been created but was")
					}

					return nil, nil
				}
			}
			if nomad, ok := transport.Nomad(); ok {
				nomad.nomadTransportBuilder = func(state *embeddedTransportNomadv1, ctx context.Context) (it.Transport, error) {
					if !test.wantNomadClient {
						t.Error("A nomad client should not have been created but was")
					}

					return nil, nil
				}
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

// helpers.
type transportconfig map[it.TransportType]configmap

func (tc transportconfig) k8s(config configmap) transportconfig {
	tc[K8S] = config.copy()
	return tc
}

func (tc transportconfig) k8sValue(key string, value interface{}) transportconfig {
	if _, ok := tc[K8S]; !ok {
		tc[K8S] = configmap{}
	}
	tc[K8S][key] = value

	return tc
}

func (tc transportconfig) ssh(config configmap) transportconfig {
	tc[SSH] = config.copy()
	return tc
}

func (tc transportconfig) sshValue(key string, value interface{}) transportconfig {
	if _, ok := tc[SSH]; !ok {
		tc[SSH] = configmap{}
	}
	tc[SSH][key] = value

	return tc
}

func (tc transportconfig) nomad(config configmap) transportconfig {
	tc[NOMAD] = config.copy()
	return tc
}

func (tc transportconfig) nomadValue(key string, value interface{}) transportconfig {
	if _, ok := tc[NOMAD]; !ok {
		tc[NOMAD] = configmap{}
	}
	tc[NOMAD][key] = value

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

	for tType, cfg := range tc {
		types[string(tType)] = tftypes.Map{ElementType: tftypes.String}
		transportValues := map[string]tftypes.Value{}
		for name, val := range cfg {
			transportValues[name] = tftypes.NewValue(tftypes.String, val)
		}
		values[string(tType)] = tftypes.NewValue(types[string(tType)], transportValues)
	}

	return tftypes.NewValue(tftypes.Object{AttributeTypes: types}, values)
}

func assertTransportCfg(t *testing.T, transport *embeddedTransportV1, config transportconfig) {
	t.Helper()

	for tType, tConfig := range config {
		if len(tConfig) > 0 {
			for attr, value := range tConfig {
				switch tType {
				case SSH:
					ssh, ok := transport.SSH()
					assert.True(t, ok)
					switch attr {
					case "host":
						assert.Equal(t, value, ssh.Host.Val)
					case "user":
						assert.Equal(t, value, ssh.User.Val)
					case "private_key":
						assert.Equal(t, value, ssh.PrivateKey.Val)
					case "private_key_path":
						assert.Equal(t, value, ssh.PrivateKeyPath.Val)
					case "passphrase":
						assert.Equal(t, value, ssh.Passphrase.Val)
					case "passphrase_path":
						assert.Equal(t, value, ssh.PassphrasePath.Val)
					default:
						t.Fatalf("unknown SSH attr: %s", attr)
					}
				case K8S:
					k8s, ok := transport.K8S()
					assert.True(t, ok)
					switch attr {
					case "kubeconfig_base64":
						assert.Equal(t, value, k8s.KubeConfigBase64.Val)
					case "context_name":
						assert.Equal(t, value, k8s.ContextName.Val)
					case "namespace":
						assert.Equal(t, value, k8s.Namespace.Val)
					case "pod":
						assert.Equal(t, value, k8s.Pod.Val)
					case "container":
						assert.Equal(t, value, k8s.Container.Val)
					default:
						t.Fatalf("unknown K8S attr: %s", attr)
					}
				case NOMAD:
					nomad, ok := transport.Nomad()
					assert.True(t, ok)
					switch attr {
					case "host":
						assert.Equal(t, value, nomad.Host.Val)
					case "secret_id":
						assert.Equal(t, value, nomad.SecretID.Val)
					case "allocation_id":
						assert.Equal(t, value, nomad.AllocationID.Val)
					case "task_name":
						assert.Equal(t, value, nomad.TaskName.Val)
					default:
						t.Fatalf("unknown Nomad attr: %s", attr)
					}
				case UNKNOWN:
					t.Fatalf("unknown transport type: %s", string(tType))
				default:
					t.Fatalf("undefined transport type: %s", string(tType))
				}
			}
		}
	}
}
