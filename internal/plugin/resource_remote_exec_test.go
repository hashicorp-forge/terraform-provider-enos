package plugin

import (
	"bytes"
	"os"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceRemoteExec tests the remote_exec resource
func TestAccResourceRemoteExec(t *testing.T) {
	var cfg = template.Must(template.New("enos_remote_exec").Parse(`resource "enos_remote_exec" "{{.ID}}" {
		{{if .Inline}}
		{{range .Inline}}
		inline = [
			"{{.}}",
		]
		{{end}}
		{{end}}

		{{if .Scripts}}
		{{range .Scripts}}
		scripts = [
			"{{.}}",
		]
		{{end}}
		{{end}}


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

	cases := []testAccResourceTemplate{}

	remoteExec := newRemoteExecStateV1()
	remoteExec.ID = "foo"
	remoteExec.Scripts = []string{"../fixtures/src.txt"}
	remoteExec.Inline = []string{"touch /tmp/foo"}
	remoteExec.Transport.SSH.User = "ubuntu"
	remoteExec.Transport.SSH.Host = "localhost"
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	remoteExec.Transport.SSH.PrivateKey = privateKey
	cases = append(cases, testAccResourceTemplate{
		"all fields are loaded correctly",
		remoteExec,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_remote_exec.foo", "id", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_remote_exec.foo", "inline[0]", regexp.MustCompile(`^/tmp/foo$`)),
			resource.TestMatchResourceAttr("enos_remote_exec.foo", "scripts[0]", regexp.MustCompile(`^../fixtures/src.txt$`)),
			resource.TestMatchResourceAttr("enos_remote_exec.foo", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_remote_exec.foo", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
		),
		false,
	})

	// To do a real test, set the environment variables when running `make testacc`
	host, ok := os.LookupEnv("ENOS_TRANSPORT_HOST")
	if !ok {
		t.Skip("SSH tests are skipped unless ENOS_TRANSPORT_* environment variables are set")
	} else {
		realTest := newRemoteExecStateV1()
		realTest.ID = "foo"
		realTest.Scripts = []string{"../fixtures/script.sh"}
		realTest.Inline = []string{"touch /tmp/foo"}
		realTest.Transport.SSH.User = os.Getenv("ENOS_TRANSPORT_USER")
		realTest.Transport.SSH.Host = host
		realTest.Transport.SSH.PrivateKeyPath = os.Getenv("ENOS_TRANSPORT_KEY_PATH")
		realTest.Transport.SSH.PassphrasePath = os.Getenv("ENOS_TRANSPORT_PASSPHRASE_PATH")
		cases = append(cases, testAccResourceTemplate{
			"real test",
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
