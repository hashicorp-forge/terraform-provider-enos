package plugin

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/enos-provider/internal/server/datarouter"
	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"

	"github.com/hashicorp/enos-provider/internal/server"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// transportRenderFunc can be used to render the embedded transport in a resource template
var transportRenderFunc = map[string]any{
	"renderTransport": func(v1 *embeddedTransportV1) (string, error) {
		return v1.render()
	},
}

func readTestFile(path string) (string, error) {
	res := ""
	abs, err := filepath.Abs(path)
	if err != nil {
		return res, err
	}

	handle, err := os.Open(abs)
	if err != nil {
		return res, err
	}
	defer handle.Close() // nolint: staticcheck

	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(handle)
	if err != nil {
		return res, err
	}

	return strings.TrimSpace(buf.String()), nil
}

type testProperty struct {
	n string
	v string
	d *tfString
}

func testMapPropertiesToStruct(props []testProperty) map[string]tftypes.Value {
	values := map[string]tftypes.Value{}

	for _, prop := range props {
		prop.d.Set(prop.v)
		values[prop.n] = prop.d.TFValue()
	}

	return values
}

// Sometimes tests will set the ENOS_ environment variables to test provider
// behavior. Always store whatever they are when the tests load so that we can
// revert them as needed.
var startEnv = func() map[string]string {
	enosVars := map[string]string{}
	enosReg := regexp.MustCompile("^ENOS_")

	for _, eVar := range os.Environ() {
		parts := strings.Split(eVar, "=")

		if enosReg.MatchString(parts[0]) {
			enosVars[parts[0]] = parts[1]
		}
	}

	return enosVars
}()

func resetEnv(t *testing.T) {
	unsetAllEnosEnv(t)

	for key, val := range startEnv {
		assert.NoError(t, os.Setenv(key, val))
	}
}

func unsetAllEnosEnv(t *testing.T) {
	t.Helper()
	unsetSSHEnv(t)
	unsetK8SEnv(t)
	unsetNomadEnv(t)
	unsetProviderEnv(t)
}

func unsetSSHEnv(t *testing.T) {
	for _, eVar := range []string{
		"ENOS_TRANSPORT_USER",
		"ENOS_TRANSPORT_HOST",
		"ENOS_TRANSPORT_PRIVATE_KEY",
		"ENOS_TRANSPORT_PRIVATE_KEY_PATH",
		"ENOS_TRANSPORT_PASSPHRASE",
		"ENOS_TRANSPORT_PASSPHRASE_PATH",
	} {
		assert.NoError(t, os.Unsetenv(eVar))
	}
}

func unsetK8SEnv(t *testing.T) {
	for _, eVar := range []string{
		"ENOS_KUBECONFIG",
		"ENOS_K8S_CONTEXT_NAME",
		"ENOS_K8S_NAMESPACE",
		"ENOS_K8S_POD",
		"ENOS_K8S_CONTAINER",
	} {
		assert.NoError(t, os.Unsetenv(eVar))
	}
}

func unsetNomadEnv(t *testing.T) {
	for _, eVar := range []string{
		"ENOS_NOMAD_HOST",
		"ENOS_NOMAD_SECRET_ID",
		"ENOS_NOMAD_ALLOCATION_ID",
		"ENOS_NOMAD_TASK_NAME",
	} {
		assert.NoError(t, os.Unsetenv(eVar))
	}
}

func unsetProviderEnv(t *testing.T) {
	assert.NoError(t, os.Unsetenv(enosDebugDataRootDirEnvVarKey))
}

func setEnosSSHEnv(t *testing.T, et *embeddedTransportV1) {
	t.Helper()

	ssh, ok := et.SSH()
	assert.True(t, ok)
	setEnvVars(t, map[string]*tfString{
		"ENOS_TRANSPORT_USER":             ssh.User,
		"ENOS_TRANSPORT_HOST":             ssh.Host,
		"ENOS_TRANSPORT_PRIVATE_KEY":      ssh.PrivateKey,
		"ENOS_TRANSPORT_PRIVATE_KEY_PATH": ssh.PrivateKeyPath,
		"ENOS_TRANSPORT_PASSPHRASE":       ssh.Passphrase,
		"ENOS_TRANSPORT_PASSPHRASE_PATH":  ssh.PassphrasePath,
	})
}

func setEnosK8SEnv(t *testing.T, et *embeddedTransportV1) {
	t.Helper()

	k8s, ok := et.K8S()
	assert.True(t, ok)
	setEnvVars(t, map[string]*tfString{
		"ENOS_KUBECONFIG":       k8s.KubeConfigBase64,
		"ENOS_K8S_CONTEXT_NAME": k8s.ContextName,
		"ENOS_K8S_NAMESPACE":    k8s.Namespace,
		"ENOS_K8S_POD":          k8s.Pod,
		"ENOS_K8S_CONTAINER":    k8s.Container,
	})
}

func setENosNomadEnv(t *testing.T, et *embeddedTransportV1) {
	t.Helper()

	nomad, ok := et.Nomad()
	assert.True(t, ok)
	setEnvVars(t, map[string]*tfString{
		"ENOS_NOMAD_HOST":          nomad.Host,
		"ENOS_NOMAD_SECRET_ID":     nomad.SecretID,
		"ENOS_NOMAD_ALLOCATION_ID": nomad.AllocationID,
		"ENOS_NOMAD_TASK_NAME":     nomad.TaskName,
	})
}

func setEnvVars(t *testing.T, vars map[string]*tfString) {
	t.Helper()
	for key, val := range vars {
		v, ok := val.Get()
		if ok {
			assert.NoError(t, os.Setenv(key, v))
		}
	}
}

func ensureEnosTransportEnvVars(t *testing.T) (map[string]string, bool) {
	t.Helper()

	var okacc, okuser, okhost, okpath bool
	vars := map[string]string{}

	_, okacc = os.LookupEnv("TF_ACC")
	vars["username"], okuser = os.LookupEnv("ENOS_TRANSPORT_USER")
	vars["host"], okhost = os.LookupEnv("ENOS_TRANSPORT_HOST")
	vars["path"], okpath = os.LookupEnv("ENOS_TRANSPORT_PRIVATE_KEY_PATH")

	if !(okacc && okuser && okhost && okpath) {
		t.Log(`skipping because TF_ACC, ENOS_TRANSPORT_USER, ENOS_TRANSPORT_HOST, and ENOS_TRANSPORT_PRIVATE_KEY_PATH environment variables need to be set`)
		t.Skip()
		return vars, false
	}

	return vars, true
}

func configureK8STransportFromEnvironment(em *embeddedTransportK8Sv1) {
	for _, key := range []struct {
		name string
		env  string
		dst  *tfString
	}{
		{"kubeconfig_base64", "ENOS_KUBECONFIG", em.KubeConfigBase64},
		{"context_name", "ENOS_K8S_CONTEXT_NAME", em.ContextName},
		{"namespace", "ENOS_K8S_NAMESPACE", em.Namespace},
		{"pod", "ENOS_K8S_POD", em.Pod},
		{"container", "ENOS_K8S_CONTAINER", em.Container},
	} {
		val, ok := os.LookupEnv(key.env)
		if ok {
			key.dst.Set(val)
			em.Values[key.name] = key.dst.TFValue()
		}
	}
}

func configureSSHTransportFromEnvironment(em *embeddedTransportSSHv1) {
	for _, key := range []struct {
		name string
		env  string
		dst  *tfString
	}{
		{"user", "ENOS_TRANSPORT_USER", em.User},
		{"host", "ENOS_TRANSPORT_HOST", em.Host},
		{"private_key", "ENOS_TRANSPORT_PRIVATE_KEY", em.PrivateKey},
		{"private_key_path", "ENOS_TRANSPORT_PRIVATE_KEY_PATH", em.PrivateKeyPath},
		{"passphrase", "ENOS_TRANSPORT_PASSPHRASE", em.Passphrase},
		{"passphrase_path", "ENOS_TRANSPORT_PASSPHRASE_PATH", em.PassphrasePath},
	} {
		val, ok := os.LookupEnv(key.env)
		if ok {
			key.dst.Set(val)
			em.Values[key.name] = key.dst.TFValue()
		}
	}
}

func configureNomadTransportFromEnvironment(em *embeddedTransportNomadv1) {
	for _, key := range []struct {
		name string
		env  string
		dst  *tfString
	}{
		{"host", "ENOS_NOMAD_USER", em.Host},
		{"secret_id", "ENOS_NOMAD_SECRET_ID", em.SecretID},
		{"allocation_id", "ENOS_NOMAD_ALLOCATION_ID", em.AllocationID},
		{"task_name", "ENOS_NOMAD_TASK_NAME", em.TaskName},
	} {
		val, ok := os.LookupEnv(key.env)
		if ok {
			key.dst.Set(val)
			em.Values[key.name] = key.dst.TFValue()
		}
	}
}

// providerOverrides can be used to provide an alternate datasource or resource to override the defaults.
type providerOverrides struct {
	datasources []datarouter.DataSource
	resources   []resourcerouter.Resource
}

// Creates a ProtoV6ProviderFactories that wraps that can be used to create a ProviderServer that has
// transport configuration injected from the environment. The providers argument is variadic only to make it
// optional, passing more than on overrides provider is not necessary.
func testProviders(t *testing.T, overrides ...providerOverrides) map[string]func() (tfprotov6.ProviderServer, error) {
	t.Helper()

	provider := newProvider()
	assert.NoError(t, provider.config.Transport.SetTransportState(newEmbeddedTransportK8Sv1(), newEmbeddedTransportSSH(), newEmbeddedTransportNomadv1()))
	if k8S, ok := provider.config.Transport.K8S(); ok {
		configureK8STransportFromEnvironment(k8S)
	}
	if ssh, ok := provider.config.Transport.SSH(); ok {
		configureSSHTransportFromEnvironment(ssh)
	}
	if nomad, ok := provider.config.Transport.Nomad(); ok {
		configureNomadTransportFromEnvironment(nomad)
	}

	var datasourceOverrides []datarouter.DataSource
	var resourceOverrides []resourcerouter.Resource
	for _, override := range overrides {
		datasourceOverrides = append(datasourceOverrides, override.datasources...)
		resourceOverrides = append(resourceOverrides, override.resources...)
	}

	return map[string]func() (tfprotov6.ProviderServer, error){
		"enos": func() (tfprotov6.ProviderServer, error) {
			return server.New(
				server.RegisterProvider(provider),
				WithDefaultDataRouter(datasourceOverrides...),
				WithDefaultResourceRouter(resourceOverrides...),
			), nil
		},
	}
}
