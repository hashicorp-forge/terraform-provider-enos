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
	cfg := template.Must(template.New("enos_remote_exec").Parse(`resource "enos_remote_exec" "{{.ID}}" {
		{{if .Content}}
		content = <<EOF
{{.Content}}
EOF
		{{end}}

		{{if .Inline}}
		inline = [
		{{range .Inline}}
			"{{.}}",
		{{end}}
		]
		{{end}}

		{{if .Scripts}}
		scripts = [
		{{range .Scripts}}
			"{{.}}",
		{{end}}
		]
		{{end}}

		{{if .Env}}
		environment = {
		{{range $name, $val := .Env}}
			"{{$name}}": "{{$val}}",
		{{end}}
		}
		{{end}}

		transport = {
			ssh = {
				{{if .Transport.SSH.User}}
				user = "{{.Transport.SSH.User}}"
				{{end}}

				{{if .Transport.SSH.Host}}
				host = "{{.Transport.SSH.Host}}"
				{{end}}

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
	remoteExec.Env = map[string]string{"FOO": "BAR"}
	remoteExec.Scripts = []string{"../fixtures/src.txt"}
	remoteExec.Inline = []string{"touch /tmp/foo"}
	remoteExec.Content = "some content"
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
			resource.TestMatchResourceAttr("enos_remote_exec.foo", "stdout", regexp.MustCompile(`^$`)),
			resource.TestMatchResourceAttr("enos_remote_exec.foo", "stderr", regexp.MustCompile(`^$`)),
			resource.TestMatchResourceAttr("enos_remote_exec.foo", "environment[FOO]", regexp.MustCompile(`^BAR$`)),
			resource.TestMatchResourceAttr("enos_remote_exec.foo", "inline[0]", regexp.MustCompile(`^/tmp/foo$`)),
			resource.TestMatchResourceAttr("enos_remote_exec.foo", "scripts[0]", regexp.MustCompile(`^../fixtures/src.txt$`)),
			resource.TestMatchResourceAttr("enos_remote_exec.foo", "content", regexp.MustCompile(`^some content$`)),
			resource.TestMatchResourceAttr("enos_remote_exec.foo", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_remote_exec.foo", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
		),
		false,
	})

	// To do a real test, set the environment variables when running `make testacc`
	host, ok := os.LookupEnv("ENOS_TRANSPORT_HOST")
	if !ok {
		t.Log("SSH tests are skipped unless ENOS_TRANSPORT_* environment variables are set")
	} else {
		realTest := newRemoteExecStateV1()
		realTest.ID = "foo"
		realTest.Env = map[string]string{"FOO": "BAR"}
		realTest.Scripts = []string{"../fixtures/script.sh"}
		realTest.Inline = []string{"touch /tmp/foo && rm /tmp/foo"}
		realTest.Content = `echo "hello world" > /tmp/enos_remote_exec_script_content`
		realTest.Transport.SSH.Host = host
		cases = append(cases, testAccResourceTemplate{
			"real test",
			realTest,
			resource.ComposeTestCheckFunc(),
			true,
		})
		noStdoutOrStderr := newRemoteExecStateV1()
		noStdoutOrStderr.ID = "foo"
		noStdoutOrStderr.Inline = []string{"exit 0"}
		noStdoutOrStderr.Transport.SSH.Host = host
		cases = append(cases, testAccResourceTemplate{
			"NoStdoutOrStderr",
			noStdoutOrStderr,
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
