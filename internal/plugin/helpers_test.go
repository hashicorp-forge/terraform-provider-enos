package plugin

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func readTestFile(path string) (string, error) {
	res := ""
	abs, err := filepath.Abs(path)
	if err != nil {
		return res, err
	}

	handle, err := os.Open(abs)
	defer handle.Close() // nolint: staticcheck
	if err != nil {
		return res, err
	}

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
	d *string
}

func testMapPropertiesToStruct(props []testProperty) map[string]tftypes.Value {
	values := map[string]tftypes.Value{}

	for _, prop := range props {
		*prop.d = prop.v
		values[prop.n] = tfMarshalStringValue(prop.v)
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
	unsetEnosEnv(t)

	for key, val := range startEnv {
		assert.NoError(t, os.Setenv(key, val))
	}
}

func unsetEnosEnv(t *testing.T) {
	t.Helper()

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

func setEnosEnv(t *testing.T, et *embeddedTransportV1) {
	t.Helper()

	for key, val := range map[string]string{
		"ENOS_TRANSPORT_USER":             et.SSH.User,
		"ENOS_TRANSPORT_HOST":             et.SSH.Host,
		"ENOS_TRANSPORT_PRIVATE_KEY":      et.SSH.PrivateKey,
		"ENOS_TRANSPORT_PRIVATE_KEY_PATH": et.SSH.PrivateKeyPath,
		"ENOS_TRANSPORT_PASSPHRASE":       et.SSH.Passphrase,
		"ENOS_TRANSPORT_PASSPHRASE_PATH":  et.SSH.PassphrasePath,
	} {
		if val != "" {
			assert.NoError(t, os.Setenv(key, val))
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
