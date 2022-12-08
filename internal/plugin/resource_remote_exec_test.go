package plugin

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
	"github.com/hashicorp/enos-provider/internal/transport/ssh"

	"github.com/hashicorp/enos-provider/internal/transport/mock"

	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	it "github.com/hashicorp/enos-provider/internal/transport"

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

func TestChangedEnvVars(t *testing.T) {
	cfg1 := template.Must(template.New("enos_remote_exec").
		Funcs(transportRenderFunc).
		Parse(`resource "enos_remote_exec" "{{.ID.Value}}" {

        environment = {
            "host" = "127.0.0.1"
            "name" = "yoyo"
        }

		content = "echo hello"

		{{ renderTransport .Transport }}
	}`))

	cfg2 := template.Must(template.New("enos_remote_exec").
		Funcs(transportRenderFunc).
		Parse(`resource "enos_remote_exec" "{{.ID.Value}}" {

        environment = {
            "host" = "10.0.6.9"
            "name" = "yoyo"
        }

		content = "echo hello"

		{{ renderTransport .Transport }}
	}`))

	remoteExecState := newRemoteExecStateV1()
	remoteExecState.ID.Set("test_changed_env")
	transportSSH := newEmbeddedTransportSSH()
	transportSSH.User.Set("ubuntu")
	transportSSH.Host.Set("127.0.0.1")
	transportSSH.PrivateKey.Set("not a private key")
	assert.NoError(t, remoteExecState.Transport.SetTransportState(transportSSH))

	var clientCreateCount int

	remoteExecResource := newRemoteExec()
	remoteExecResource.stateFactory = func() *remoteExecStateV1 {
		remoteExecState := newRemoteExecStateV1()
		embeddedTransport := newEmbeddedTransport()

		embeddedTransport.clientFactory = func(ctx context.Context, transport transportState) (it.Transport, error) {
			clientCreateCount = clientCreateCount + 1
			return mock.New(), nil
		}
		remoteExecState.Transport = embeddedTransport

		return remoteExecState
	}

	providers := testProviders(t, providerOverrides{resources: []resourcerouter.Resource{remoteExecResource}})

	s1 := bytes.Buffer{}
	err := cfg1.Execute(&s1, remoteExecState)
	if err != nil {
		t.Fatalf("error executing test template: %s", err.Error())
	}

	apply1 := resource.TestStep{
		Config:   s1.String(),
		PlanOnly: false,
	}

	s2 := bytes.Buffer{}
	err = cfg2.Execute(&s2, remoteExecState)
	if err != nil {
		t.Fatalf("error executing test template: %s", err.Error())
	}

	apply2 := resource.TestStep{
		Config:   s2.String(),
		PlanOnly: false,
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providers,
		Steps: []resource.TestStep{
			apply1,
			apply2,
		},
	})

	assert.Equal(t, 2, clientCreateCount)
}

func TestInlineWithPipedCommandAndEnvVars(t *testing.T) {

	host, hostOk := os.LookupEnv("ENOS_TRANSPORT_HOST")
	privateKeyPath, keyOk := os.LookupEnv("ENOS_TRANSPORT_PRIVATE_KEY_PATH")
	if !hostOk || !keyOk {
		t.Skip("Test skipped since either ENOS_TRANSPORT_HOST and ENOS_TRANSPORT_PRIVATE_KEY_PATH environment variables are not set")
	}

	token := "eyJhbGciOiJFUzUxMiIsInR5cCI6IkpXVCJ9.eyJhY2Nlc3NvciI6IiIsImFkZHIiOiJodHRwOi8vMTAuMTMuMTIuMjE1OjgyMDAiLCJleHAiOjE2NzA1MTc1MTIsImlhdCI6MTY3MDUxNTcxMiwianRpIjoiaHZzLnEySmFXSzR4SER3algwVDZxQWJ3MkltYiIsIm5iZiI6MTY3MDUxNTcwNywidHlwZSI6IndyYXBwaW5nIn0.AXahR22v_CEESZBCk6N8YMYnoKSgjcEl-HI9-8n0pKXkQ9qXK50di8YcGiVspuMMdjPOZsEWy7N3KLXAKq4H7008AalNaqtuYPdR3f34dXo7c1DScepN1sURKZLV8xMbcsgnDa_4h_1ROmHVnObrOCoy2nZ-vsDB6CXHuxgTc7x5x-dM"
	tokens := fmt.Sprintf(`Key                              Value
---                              -----
wrapping_token:                  %s
wrapping_accessor:               NhPTqq7xRTdlG2a9kQbdVhy4
wrapping_token_ttl:              30m
wrapping_token_creation_time:    2022-12-08 16:08:32.532833589 +0000 UTC
wrapping_token_creation_path:    sys/replication/performance/primary/secondary-token`, token)

	client, err := ssh.New(ssh.WithHost(host), ssh.WithKeyPath(privateKeyPath), ssh.WithUser("ubuntu"))
	assert.NoError(t, err)

	tokensFile := "/tmp/tokens"
	assert.NoError(t, client.Copy(context.Background(), tfile.NewReader(tokens), tokensFile))

	cfg := template.Must(template.New("enos_remote_exec").
		Funcs(transportRenderFunc).
		Parse(`resource "enos_remote_exec" "inline_with_piped_command_and_env_vars" {

        environment = {
            "TOKENS_FILE": "{{ .TokensFile }}"
        }

		inline = ["cat $TOKENS_FILE |sed -n '/^wrapping_token:/p' |awk '{print $2}'"]

		{{ renderTransport .Transport }}
	}`))

	transport := newEmbeddedTransport()
	transportSSH := newEmbeddedTransportSSH()
	transportSSH.User.Set("ubuntu")
	transportSSH.Host.Set(host)
	transportSSH.PrivateKeyPath.Set(privateKeyPath)
	assert.NoError(t, transport.SetTransportState(transportSSH))
	data := map[string]interface{}{
		"TokensFile": tokensFile,
		"Transport":  transport,
	}

	s := bytes.Buffer{}
	assert.NoError(t, cfg.Execute(&s, data))

	apply := resource.TestStep{
		Config:   s.String(),
		PlanOnly: false,
		Check:    resource.TestCheckResourceAttr("enos_remote_exec.inline_with_piped_command_and_env_vars", "stdout", token),
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviders(t),
		Steps: []resource.TestStep{
			apply,
		},
	})
}
