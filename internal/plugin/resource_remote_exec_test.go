package plugin

import (
	"bytes"
	"os"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/enos-provider/internal/server/state"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceRemoteExec tests the remote_exec resource
func TestAccResourceRemoteExec(t *testing.T) {
	cfg := template.Must(template.New("enos_remote_exec").
		Funcs(transportRenderFunc).
		Parse(`resource "enos_remote_exec" "{{.ID.Value}}" {
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

		{{ renderTransport .Transport }}
	}`))

	cases := []testAccResourceTemplate{}

	remoteExec := newRemoteExecStateV1()
	remoteExec.ID.Set("foo")
	remoteExec.Env.SetStrings(map[string]string{"FOO": "BAR"})
	remoteExec.Scripts.SetStrings([]string{"../fixtures/src.txt"})
	remoteExec.Inline.SetStrings([]string{"touch /tmp/foo"})
	remoteExec.Content.Set("some content")
	ssh := newEmbeddedTransportSSH()
	ssh.User.Set("ubuntu")
	ssh.Host.Set("localhost")
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	ssh.PrivateKey.Set(privateKey)
	assert.NoError(t, remoteExec.Transport.SetTransportState(ssh))
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
		ssh := newEmbeddedTransportSSH()
		ssh.Host.Set(host)
		assert.NoError(t, realTest.Transport.SetTransportState(ssh))
		cases = append(cases, testAccResourceTemplate{
			"real test",
			realTest,
			resource.ComposeTestCheckFunc(),
			true,
		})
		noStdoutOrStderr := newRemoteExecStateV1()
		noStdoutOrStderr.ID.Set("foo")
		noStdoutOrStderr.Inline.SetStrings([]string{"exit 0"})
		ssh = newEmbeddedTransportSSH()
		ssh.Host.Set(host)
		assert.NoError(t, noStdoutOrStderr.Transport.SetTransportState(ssh))
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
				ssh := newEmbeddedTransportSSH()
				ssh.Host.Set(host)
				assert.NoError(t, test.Transport.SetTransportState(ssh))
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

func TestBadTransportConfig(t *testing.T) {
	cfg := template.Must(template.New("enos_remote_exec").
		Funcs(transportRenderFunc).
		Parse(`resource "enos_remote_exec" "{{.ID.Value}}" {
		content = "echo hello"

		{{ renderTransport .Transport }}
	}`))

	k8sRemoteExecState := newRemoteExecStateV1()
	k8sRemoteExecState.ID.Set("bad_k8s_transport")
	k8sTransport := newEmbeddedTransportK8Sv1()
	k8sTransport.KubeConfigBase64.Set("balogna")
	k8sTransport.Namespace.Set("namespace")
	k8sTransport.ContextName.Set("bananas")
	k8sTransport.Pod.Set("yoyo")
	k8sTransport.Container.Set("container")
	assert.NoError(t, k8sRemoteExecState.Transport.SetTransportState(k8sTransport))

	sshRemoteExecState := newRemoteExecStateV1()
	sshRemoteExecState.ID.Set("bad_ssh_transport")
	sshTransport := newEmbeddedTransportSSH()
	sshTransport.Host.Set("127.0.0.1")
	sshTransport.User.Set("ubuntu")
	sshTransport.PrivateKey.Set("not a key")
	sshTransport.PrivateKeyPath.Set("/not/a/real/path")
	sshTransport.Passphrase.Set("balogna")
	sshTransport.PassphrasePath.Set("/not/a/passphrase")
	assert.NoError(t, sshRemoteExecState.Transport.SetTransportState(sshTransport))

	nomadRemoteExecState := newRemoteExecStateV1()
	nomadRemoteExecState.ID.Set("bad_nomad_transport")
	nomadTransport := newEmbeddedTransportNomadv1()
	nomadTransport.Host.Set("bogus_url")
	nomadTransport.SecretID.Set("bologna")
	nomadTransport.AllocationID.Set("some id")
	nomadTransport.TaskName.Set("bananas")
	assert.NoError(t, nomadRemoteExecState.Transport.SetTransportState(nomadTransport))

	tests := []struct {
		name               string
		state              state.State
		expectedErrorRegEx *regexp.Regexp
	}{
		{
			"k8s",
			k8sRemoteExecState,
			regexp.MustCompile(`(?s:.*kubeconfig_base64 : \[redacted].*context_name : bananas.*namespace : namespace.*pod : yoyo.*container : container.*)`),
		},
		{
			"ssh",
			sshRemoteExecState,
			regexp.MustCompile(`(?s:user : ubuntu.*host : 127\.0\.0\.1.*private_key : not a key.*private_key_path : \/not\/a\/real\/path.*passphrase : \[redacted].*passphrase_path : \/not\/a\/passphrase)`),
		},
		{
			"nomad",
			nomadRemoteExecState,
			regexp.MustCompile(`(?s:host : bogus_url.*secret_id : \[redacted].*allocation_id : some id.*task_name : bananas)`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := bytes.Buffer{}
			err := cfg.Execute(&buf, test.state)
			if err != nil {
				t.Fatalf("error executing test template: %s", err.Error())
			}

			step := resource.TestStep{
				Config:      buf.String(),
				PlanOnly:    false,
				ExpectError: test.expectedErrorRegEx,
			}

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testProviders(t),
				Steps:                    []resource.TestStep{step},
			})

			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testProviders(t),
				Steps:                    []resource.TestStep{step},
			})
		})
	}
}
