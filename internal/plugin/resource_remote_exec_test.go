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
	cfg := template.Must(template.New("enos_remote_exec").Parse(`resource "enos_remote_exec" "{{.ID.Value}}" {
		{{if .Content.Value}}
		content = <<EOF
{{.Content.Value}}
EOF
		{{end}}

		{{if .Inline.StringValue}}
		inline = [
		{{range .Inline.StringValue}}
			"{{.}}",
		{{end}}
		]
		{{end}}

		{{if .Scripts.StringValue}}
		scripts = [
		{{range .Scripts.StringValue}}
			"{{.}}",
		{{end}}
		]
		{{end}}

		{{if .Env.StringValue}}
		environment = {
		{{range $name, $val := .Env.StringValue}}
			"{{$name}}": "{{$val}}",
		{{end}}
		}
		{{end}}

		transport = {
			ssh = {
				{{if .Transport.SSH.User.Value}}
				user = "{{.Transport.SSH.User.Value}}"
				{{end}}

				{{if .Transport.SSH.Host.Value}}
				host = "{{.Transport.SSH.Host.Value}}"
				{{end}}

				{{if .Transport.SSH.PrivateKey.Value}}
				private_key = <<EOF
{{.Transport.SSH.PrivateKey.Value}}
EOF
				{{end}}

				{{if .Transport.SSH.PrivateKeyPath.Value}}
				private_key_path = "{{.Transport.SSH.PrivateKeyPath.Value}}"
				{{end}}

				{{if .Transport.SSH.Passphrase.Value}}
				passphrase = "{{.Transport.SSH.Passphrase.Value}}"
				{{end}}

				{{if .Transport.SSH.PassphrasePath.Value}}
				passphrase_path = "{{.Transport.SSH.PassphrasePath.Value}}"
				{{end}}
			}
		}
	}`))

	cases := []testAccResourceTemplate{}

	remoteExec := newRemoteExecStateV1()
	remoteExec.ID.Set("foo")
	remoteExec.Env.SetStrings(map[string]string{"FOO": "BAR"})
	remoteExec.Scripts.SetStrings([]string{"../fixtures/src.txt"})
	remoteExec.Inline.SetStrings([]string{"touch /tmp/foo"})
	remoteExec.Content.Set("some content")
	remoteExec.Transport.SSH.User.Set("ubuntu")
	remoteExec.Transport.SSH.Host.Set("localhost")
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	remoteExec.Transport.SSH.PrivateKey.Set(privateKey)
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
		realTest.ID.Set("foo")
		realTest.Env.SetStrings(map[string]string{"FOO": "BAR"})
		realTest.Scripts.SetStrings([]string{"../fixtures/script.sh"})
		realTest.Inline.SetStrings([]string{"touch /tmp/foo && rm /tmp/foo"})
		realTest.Content.Set(`echo "hello world" > /tmp/enos_remote_exec_script_content`)
		realTest.Transport.SSH.Host.Set(host)
		cases = append(cases, testAccResourceTemplate{
			"real test",
			realTest,
			resource.ComposeTestCheckFunc(),
			true,
		})
		noStdoutOrStderr := newRemoteExecStateV1()
		noStdoutOrStderr.ID.Set("foo")
		noStdoutOrStderr.Inline.SetStrings([]string{"exit 0"})
		noStdoutOrStderr.Transport.SSH.Host.Set(host)
		cases = append(cases, testAccResourceTemplate{
			"NoStdoutOrStderr",
			noStdoutOrStderr,
			resource.ComposeTestCheckFunc(),
			true,
		})

		t.Run("CanHandleUpdatedAttributesAndOutput", func(t *testing.T) {
			steps := []resource.TestStep{}

			for _, cmd := range []string{
				"exit 0",
				"echo 'stderr' 1>&2",
				"echo 'stderr' 1>&2",
				"echo 'stdout' && echo 'stderr' 1>&2",
				"echo 'stdout' && echo 'stderr' 1>&2",
				"exit 0",
			} {
				test := newRemoteExecStateV1()
				test.ID.Set("foo")
				test.Content.Set(cmd)
				test.Transport.SSH.Host.Set(host)
				buf := bytes.Buffer{}
				err := cfg.Execute(&buf, test)
				if err != nil {
					t.Fatalf("error executing test template: %s", err.Error())
				}

				steps = append(steps, resource.TestStep{
					Config: buf.String(),
				})
			}

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testProviders(t),
				Steps:                    steps,
			})
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

			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testProviders(t),
				Steps:                    []resource.TestStep{step},
			})
		})
	}
}
