package plugin

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/enos-provider/internal/server"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

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

func setEnosSSHEnv(t *testing.T, et *embeddedTransportV1) {
	t.Helper()

	setEnvVars(t, map[string]*tfString{
		"ENOS_TRANSPORT_USER":             et.SSH.User,
		"ENOS_TRANSPORT_HOST":             et.SSH.Host,
		"ENOS_TRANSPORT_PRIVATE_KEY":      et.SSH.PrivateKey,
		"ENOS_TRANSPORT_PRIVATE_KEY_PATH": et.SSH.PrivateKeyPath,
		"ENOS_TRANSPORT_PASSPHRASE":       et.SSH.Passphrase,
		"ENOS_TRANSPORT_PASSPHRASE_PATH":  et.SSH.PassphrasePath,
	})
}

func setEnosK8SEnv(t *testing.T, et *embeddedTransportV1) {
	t.Helper()

	setEnvVars(t, map[string]*tfString{
		"ENOS_KUBECONFIG":       et.K8S.KubeConfig,
		"ENOS_K8S_CONTEXT_NAME": et.K8S.ContextName,
		"ENOS_K8S_NAMESPACE":    et.K8S.Namespace,
		"ENOS_K8S_POD":          et.K8S.Pod,
		"ENOS_K8S_CONTAINER":    et.K8S.Container,
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
		{"kubeconfig", "ENOS_KUBECONFIG", em.KubeConfig},
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

// Creates a ProtoV6ProviderFactories that wraps that can be used to create a ProviderServer that has
// transport configuration injected from the environment.
func testProviders(t *testing.T) map[string]func() (tfprotov6.ProviderServer, error) {
	t.Helper()

	provider := newProvider()
	configureK8STransportFromEnvironment(provider.config.Transport.K8S)
	configureSSHTransportFromEnvironment(provider.config.Transport.SSH)

	return map[string]func() (tfprotov6.ProviderServer, error){
		"enos": func() (tfprotov6.ProviderServer, error) {
			return server.New(
				server.RegisterProvider(provider),
				WithDefaultDataRouter(),
				WithDefaultResourceRouter(),
			), nil
		},
	}
}
