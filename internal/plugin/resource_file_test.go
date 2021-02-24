package plugin

import (
	"bytes"
	"os"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

type testAccResourceFileCase struct {
	name  string
	state State
	check resource.TestCheckFunc
	apply bool
}

// TestAccResourceFileTransport tests both the basic enos_file resource interface
// but also the embedded transport interface. As the embedded transport isn't
// an actual resource we're doing it here.
func TestAccResourceFileTransport(t *testing.T) {
	var cfg = template.Must(template.New("enos_file").Parse(`resource "enos_file" "{{.ID}}" {
		source = "{{.Src}}"
		destination = "{{.Dst}}"

		transport = {
			ssh = {
				user = "{{.Transport.SSH.User}}"
				host = "{{.Transport.SSH.Host}}"

				{{if .Transport.SSH.PrivateKey}}
				private_key = <<EOF
{{.Transport.SSH.PrivateKey}}
EOF
				{{end}}

				{{if .Transport.SSH.PrivateKeyPath}}
				private_key_path = "{{.Transport.SSH.PrivateKeyPath}}"
				{{end}}

				{{if .Transport.SSH.Passphrase}}
				passphrase = "{{.Transport.SSH.Passphrase}}"
				{{end}}

				{{if .Transport.SSH.PassphrasePath}}
				passphrase_path = "{{.Transport.SSH.PassphrasePath}}"
				{{end}}
			}
		}
	}`))

	cases := []testAccResourceFileCase{}

	keyNoPass := newFileState()
	keyNoPass.ID = "foo"
	keyNoPass.Src = "../fixtures/src.txt"
	keyNoPass.Dst = "/tmp/dst"
	keyNoPass.Transport.SSH.User = "ubuntu"
	keyNoPass.Transport.SSH.Host = "localhost"
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	keyNoPass.Transport.SSH.PrivateKey = privateKey
	cases = append(cases, testAccResourceFileCase{
		"private key value with no passphrase",
		keyNoPass,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_file.foo", "id", regexp.MustCompile(`^static$`)),
			resource.TestMatchResourceAttr("enos_file.foo", "source", regexp.MustCompile(`^/tmp/foo$`)),
			resource.TestMatchResourceAttr("enos_file.foo", "destination", regexp.MustCompile(`^/tmp/foo$`)),
			resource.TestMatchResourceAttr("enos_file.foo", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_file.foo", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
		),
		false,
	})

	keyPathNoPass := newFileState()
	keyPathNoPass.ID = "foo"
	keyPathNoPass.Src = "../fixtures/src.txt"
	keyPathNoPass.Dst = "/tmp/dst"
	keyPathNoPass.Transport.SSH.User = "ubuntu"
	keyPathNoPass.Transport.SSH.Host = "localhost"
	keyPathNoPass.Transport.SSH.PrivateKeyPath = "../fixtures/ssh.pem"
	cases = append(cases, testAccResourceFileCase{
		"private key from a file path with no passphrase",
		keyPathNoPass,
		resource.ComposeTestCheckFunc(),
		false,
	})

	keyPass := newFileState()
	keyPass.ID = "foo"
	keyPass.Src = "../fixtures/src.txt"
	keyPass.Dst = "/tmp/dst"
	keyPass.Transport.SSH.User = "ubuntu"
	keyPass.Transport.SSH.Host = "localhost"
	keyPass.Transport.SSH.PrivateKeyPath = "../fixtures/ssh_pass.pem"
	passphrase, err := readTestFile("../fixtures/passphrase.txt")
	require.NoError(t, err)
	keyPass.Transport.SSH.Passphrase = passphrase
	cases = append(cases, testAccResourceFileCase{
		"private key value with passphrase value",
		keyPass,
		resource.ComposeTestCheckFunc(),
		false,
	})

	keyPassPath := newFileState()
	keyPassPath.ID = "foo"
	keyPassPath.Src = "../fixtures/src.txt"
	keyPassPath.Dst = "/tmp/dst"
	keyPassPath.Transport.SSH.User = "ubuntu"
	keyPassPath.Transport.SSH.Host = "localhost"
	keyPassPath.Transport.SSH.PrivateKeyPath = "../fixtures/ssh_pass.pem"
	keyPassPath.Transport.SSH.PassphrasePath = "../fixtures/passphrase.txt"
	cases = append(cases, testAccResourceFileCase{
		"private key value with passphrase from file path",
		keyPassPath,
		resource.ComposeTestCheckFunc(),
		false,
	})

	// To do a real test, set the environment variables when running `make testacc`
	host, ok := os.LookupEnv("ENOS_TRANSPORT_HOST")
	if ok {
		realTest := newFileState()
		realTest.ID = "real"
		realTest.Src = "../fixtures/src.txt"
		realTest.Dst = "/tmp/dst"
		realTest.Transport.SSH.User = os.Getenv("ENOS_TRANSPORT_USER")
		realTest.Transport.SSH.Host = host
		realTest.Transport.SSH.PrivateKeyPath = os.Getenv("ENOS_TRANSPORT_KEY_PATH")
		realTest.Transport.SSH.PassphrasePath = os.Getenv("ENOS_TRANSPORT_PASSPHRASE_PATH")
		cases = append(cases, testAccResourceFileCase{
			"real_test",
			realTest,
			resource.ComposeTestCheckFunc(),
			true,
		})
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			buf := bytes.Buffer{}
			err := cfg.Execute(&buf, test.state)
			if err != nil {
				t.Fatalf("error executing test template: %s", err.Error())
			}

			step := resource.TestStep{
				Config: buf.String(),
				Check:  test.check,
			}

			if !test.apply {
				step.PlanOnly = true
				step.ExpectNonEmptyPlan = true
			}

			resource.Test(t, resource.TestCase{
				ProtoV5ProviderFactories: testProviders,
				Steps:                    []resource.TestStep{step},
			})
		})
	}
}

func TestResourceFileMarshalRoundtrip(t *testing.T) {
	state := newFileState()
	state.ID = "foo"
	state.Src = "/tmp/src"
	state.Dst = "/tmp/dst"
	state.Transport.SSH.User = "ubuntu"
	state.Transport.SSH.Host = "localhost"
	state.Transport.SSH.PrivateKey = "PRIVATE KEY"
	state.Transport.SSH.PrivateKeyPath = "/path/to/key.pem"

	marshaled, err := marshal(state)
	require.NoError(t, err)

	newState := newFileState()
	err = unmarshal(newState, marshaled)
	require.NoError(t, err)

	assert.Equal(t, state.ID, newState.ID)
	assert.Equal(t, state.Src, newState.Src)
	assert.Equal(t, state.Dst, newState.Dst)
	assert.Equal(t, state.Transport.SSH.User, newState.Transport.SSH.User)
	assert.Equal(t, state.Transport.SSH.Host, newState.Transport.SSH.Host)
	assert.Equal(t, state.Transport.SSH.PrivateKey, newState.Transport.SSH.PrivateKey)
	assert.Equal(t, state.Transport.SSH.PrivateKeyPath, newState.Transport.SSH.PrivateKeyPath)
}
