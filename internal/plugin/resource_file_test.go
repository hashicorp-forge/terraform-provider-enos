package plugin

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"testing"
	"text/template"

	state "github.com/hashicorp/enos-provider/internal/server/state"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testAccResourceTemplate struct {
	name  string
	state state.State
	check resource.TestCheckFunc
	apply bool
}

type testAccResourceTransportTemplate struct {
	name             string
	state            state.State
	check            resource.TestCheckFunc
	transport        *embeddedTransportV1
	resourceTemplate *template.Template
	transportUsed    it.TransportType
}

// TestAccResourceFileResourceTransport tests both the basic enos_file resource interface
// but also the embedded transport interface. As the embedded transport isn't
// an actual resource we're doing it here.
//
//nolint:paralleltest// because we modify the environment
func TestAccResourceFileResourceTransport(t *testing.T) {
	defer resetEnv(t)

	providerTransport := template.Must(template.New("enos_file").Parse(`resource "enos_file" "{{.ID.Value}}" {
		{{if .Src.Value}}
		source = "{{.Src.Value}}"
		{{end}}

		{{if .Content.Value}}
		content = <<EOF
{{.Content.Value}}
EOF
		{{end}}

		destination = "{{.Dst.Value}}"
	}`))

	resourceTransport := template.Must(template.New("enos_file").
		Funcs(transportRenderFunc).
		Parse(`resource "enos_file" "{{.ID.Value}}" {
			{{if .Src.Value}}
			source = "{{.Src.Value}}"
			{{end}}

			{{if .Content.Value}}
			content = <<EOF
	{{.Content.Value}}"
	EOF
			{{end}}

			destination = "{{.Dst.Value}}"

			{{ renderTransport .Transport }}
		}`))

	cases := []testAccResourceTransportTemplate{}

	keyNoPass := newFileState()
	keyNoPass.ID.Set("foo")
	keyNoPass.Src.Set("../fixtures/src.txt")
	keyNoPass.Dst.Set("/tmp/dst")
	ssh := newEmbeddedTransportSSH()
	ssh.User.Set("ubuntu")
	ssh.Host.Set("localhost")
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	ssh.PrivateKey.Set(privateKey)
	assert.NoError(t, keyNoPass.Transport.SetTransportState(ssh))
	cases = append(cases, testAccResourceTransportTemplate{
		"[ssh] private key value with no passphrase",
		keyNoPass,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_file.foo", "id", regexp.MustCompile(`^static$`)),
			resource.TestMatchResourceAttr("enos_file.foo", "source", regexp.MustCompile(`^/tmp/foo$`)),
			resource.TestMatchResourceAttr("enos_file.foo", "destination", regexp.MustCompile(`^/tmp/foo$`)),
			resource.TestMatchResourceAttr("enos_file.foo", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_file.foo", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
		),
		keyNoPass.Transport,
		resourceTransport,
		SSH,
	})

	keyPathNoPass := newFileState()
	keyPathNoPass.ID.Set("foo")
	keyPathNoPass.Src.Set("../fixtures/src.txt")
	keyPathNoPass.Dst.Set("/tmp/dst")
	ssh = newEmbeddedTransportSSH()
	ssh.User.Set("ubuntu")
	ssh.Host.Set("localhost")
	ssh.PrivateKeyPath.Set("../fixtures/ssh.pem")
	assert.NoError(t, keyPathNoPass.Transport.SetTransportState(ssh))
	cases = append(cases, testAccResourceTransportTemplate{
		"[ssh] private key from a file path with no passphrase",
		keyPathNoPass,
		resource.ComposeTestCheckFunc(),
		keyPathNoPass.Transport,
		resourceTransport,
		SSH,
	})

	keyPass := newFileState()
	keyPass.ID.Set("foo")
	keyPass.Src.Set("../fixtures/src.txt")
	keyPass.Dst.Set("/tmp/dst")
	ssh = newEmbeddedTransportSSH()
	ssh.User.Set("ubuntu")
	ssh.Host.Set("localhost")
	ssh.PrivateKeyPath.Set("../fixtures/ssh_pass.pem")
	passphrase, err := readTestFile("../fixtures/passphrase.txt")
	require.NoError(t, err)
	ssh.Passphrase.Set(passphrase)
	assert.NoError(t, keyPass.Transport.SetTransportState(ssh))
	cases = append(cases, testAccResourceTransportTemplate{
		"[ssh] private key value with passphrase value",
		keyPass,
		resource.ComposeTestCheckFunc(),
		keyPass.Transport,
		resourceTransport,
		SSH,
	})

	keyPassPath := newFileState()
	keyPassPath.ID.Set("foo")
	keyPassPath.Src.Set("../fixtures/src.txt")
	keyPassPath.Dst.Set("/tmp/dst")
	ssh = newEmbeddedTransportSSH()
	ssh.User.Set("ubuntu")
	ssh.Host.Set("localhost")
	ssh.PrivateKeyPath.Set("../fixtures/ssh_pass.pem")
	ssh.PassphrasePath.Set("../fixtures/passphrase.txt")
	assert.NoError(t, keyPassPath.Transport.SetTransportState(ssh))
	cases = append(cases, testAccResourceTransportTemplate{
		"[ssh] private key value with passphrase from file path",
		keyPassPath,
		resource.ComposeTestCheckFunc(),
		keyPassPath.Transport,
		resourceTransport,
		SSH,
	})

	content := newFileState()
	content.ID.Set("foo")
	content.Content.Set("hello world")
	content.Dst.Set("/tmp/dst")
	ssh = newEmbeddedTransportSSH()
	ssh.User.Set("ubuntu")
	ssh.Host.Set("localhost")
	ssh.PrivateKeyPath.Set("../fixtures/ssh_pass.pem")
	ssh.PassphrasePath.Set("../fixtures/passphrase.txt")
	assert.NoError(t, content.Transport.SetTransportState(ssh))
	cases = append(cases, testAccResourceTransportTemplate{
		"[ssh] with string content instead of source file",
		content,
		resource.ComposeTestCheckFunc(),
		content.Transport,
		resourceTransport,
		SSH,
	})

	content = newFileState()
	content.ID.Set("foo")
	content.Content.Set("hello world")
	content.Dst.Set("/tmp/dst")
	k8s := newEmbeddedTransportK8Sv1()
	k8s.KubeConfigBase64.Set("../fixtures/kubeconfig")
	k8s.ContextName.Set("kind-kind")
	k8s.Pod.Set("some-pod")
	assert.NoError(t, content.Transport.SetTransportState(k8s))
	cases = append(cases, testAccResourceTransportTemplate{
		"[kubernetes] with string content instead of source file",
		content,
		resource.ComposeTestCheckFunc(),
		content.Transport,
		resourceTransport,
		K8S,
	})

	content = newFileState()
	content.ID.Set("foo")
	content.Content.Set("hello world")
	content.Dst.Set("/tmp/dst")
	nomad := newEmbeddedTransportNomadv1()
	nomad.Host.Set("http://127.0.0.1:4646")
	nomad.SecretID.Set("secret")
	nomad.AllocationID.Set("d76bc89d")
	nomad.TaskName.Set("task")
	assert.NoError(t, content.Transport.SetTransportState(nomad))
	cases = append(cases, testAccResourceTransportTemplate{
		"[nomad] with string content instead of source file",
		content,
		resource.ComposeTestCheckFunc(),
		content.Transport,
		resourceTransport,
		NOMAD,
	})

	content = newFileState()
	content.ID.Set("foo")
	content.Content.Set("hello world")
	content.Dst.Set("/tmp/dst")
	nomad = newEmbeddedTransportNomadv1()
	nomad.Host.Set("http://127.0.0.1:4646")
	nomad.AllocationID.Set("d76bc89d")
	nomad.TaskName.Set("task")
	assert.NoError(t, content.Transport.SetTransportState(nomad))
	cases = append(cases, testAccResourceTransportTemplate{
		"[nomad] with string content instead of source file no secret id",
		content,
		resource.ComposeTestCheckFunc(),
		content.Transport,
		resourceTransport,
		NOMAD,
	})

	for _, test := range cases {
		// Run them with resource defined transport config
		t.Run(fmt.Sprintf("resource transport %s", test.name), func(t *testing.T) {
			unsetAllEnosEnv(t)
			defer resetEnv(t)

			buf := bytes.Buffer{}
			err := test.resourceTemplate.Execute(&buf, test.state)
			if err != nil {
				t.Fatalf("error executing test template: %s", err.Error())
			}

			step := resource.TestStep{
				Config:             buf.String(),
				Check:              test.check,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			}

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testProviders(t),
				Steps:                    []resource.TestStep{step},
			})
		})

		// Run them with provider config passed through the environment
		t.Run(fmt.Sprintf("provider transport %s", test.name), func(t *testing.T) {
			unsetAllEnosEnv(t)
			switch test.transportUsed {
			case SSH:
				setEnosSSHEnv(t, test.transport)
			case K8S:
				setEnosK8SEnv(t, test.transport)
			case NOMAD:
				setENosNomadEnv(t, test.transport)
			case UNKNOWN:
				t.Errorf("unknown transport type: %s", test.transportUsed)
			default:
				t.Errorf("undefined transport type: %s", test.transportUsed)
			}
			defer resetEnv(t)

			buf := bytes.Buffer{}
			err := providerTransport.Execute(&buf, test.state)
			if err != nil {
				t.Fatalf("error executing test template: %s", err.Error())
			}

			step := resource.TestStep{
				Config:             buf.String(),
				Check:              test.check,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			}

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testProviders(t),
				Steps:                    []resource.TestStep{step},
			})
		})
	}

	resetEnv(t)
	// To do a real test, set the environment variables when running `make testacc`
	host, ok := os.LookupEnv("ENOS_TRANSPORT_HOST")
	if !ok {
		t.Log("SSH tests are skipped unless ENOS_TRANSPORT_* environment variables are set")
	} else {
		cases := []testAccResourceTransportTemplate{}

		realTestSrc := newFileState()
		realTestSrc.ID.Set("real")
		realTestSrc.Src.Set("../fixtures/src.txt")
		realTestSrc.Dst.Set("/tmp/real_test_src")
		ssh := newEmbeddedTransportSSH()
		ssh.User.Set(os.Getenv("ENOS_TRANSPORT_USER"))
		ssh.Host.Set(host)
		ssh.PrivateKeyPath.Set(os.Getenv("ENOS_TRANSPORT_PRIVATE_KEY_PATH"))
		ssh.PassphrasePath.Set(os.Getenv("ENOS_TRANSPORT_PASSPHRASE_PATH"))
		assert.NoError(t, realTestSrc.Transport.SetTransportState(ssh))
		cases = append(cases, testAccResourceTransportTemplate{
			"[ssh] real test source file",
			realTestSrc,
			resource.ComposeTestCheckFunc(),
			realTestSrc.Transport,
			resourceTransport,
			SSH,
		})

		realTestContent := newFileState()
		realTestContent.ID.Set("real")
		realTestContent.Content.Set("string")
		realTestContent.Dst.Set("/tmp/real_test_content")
		ssh = newEmbeddedTransportSSH()
		ssh.User.Set(os.Getenv("ENOS_TRANSPORT_USER"))
		ssh.Host.Set(host)
		ssh.PrivateKeyPath.Set(os.Getenv("ENOS_TRANSPORT_PRIVATE_KEY_PATH"))
		ssh.PassphrasePath.Set(os.Getenv("ENOS_TRANSPORT_PASSPHRASE_PATH"))
		assert.NoError(t, realTestContent.Transport.SetTransportState(ssh))
		cases = append(cases, testAccResourceTransportTemplate{
			"[ssh] real test content",
			realTestContent,
			resource.ComposeTestCheckFunc(),
			realTestContent.Transport,
			resourceTransport,
			SSH,
		})

		for _, test := range cases {
			// Run them with resource defined transport config
			t.Run(fmt.Sprintf("resource transport %s", test.name), func(tt *testing.T) {
				defer resetEnv(tt)

				buf := bytes.Buffer{}
				err := test.resourceTemplate.Execute(&buf, test.state)
				if err != nil {
					tt.Fatalf("error executing test template: %s", err.Error())
				}

				step := resource.TestStep{
					Config:             buf.String(),
					Check:              test.check,
					PlanOnly:           false,
					ExpectNonEmptyPlan: false,
				}

				resource.Test(tt, resource.TestCase{
					ProtoV6ProviderFactories: testProviders(tt),
					Steps:                    []resource.TestStep{step},
				})
			})

			// Run them with provider config passed through the environment
			t.Run(fmt.Sprintf("provider transport %s", test.name), func(tt *testing.T) {
				resetEnv(tt)
				unsetK8SEnv(tt) // for now we don't support real k8s based tests
				defer resetEnv(tt)

				buf := bytes.Buffer{}
				err := providerTransport.Execute(&buf, test.state)
				if err != nil {
					tt.Fatalf("error executing test template: %s", err.Error())
				}

				step := resource.TestStep{
					Config:             buf.String(),
					Check:              test.check,
					PlanOnly:           false,
					ExpectNonEmptyPlan: false,
				}

				resource.Test(tt, resource.TestCase{
					ProtoV6ProviderFactories: testProviders(tt),
					Steps:                    []resource.TestStep{step},
				})
			})
		}
	}
}

// TestResourceFileTransportInvalidAttributes ensures that we can gracefully
// handle invalid attributes in the transport configuration. Since it's a dynamic
// pseudo type we cannot rely on Terraform's built-in validation.
func TestResourceFileTransportInvalidAttributes(t *testing.T) {
	t.Parallel()

	sshCfg := `resource enos_file "bad_ssh" {
	destination = "/tmp/dst"
	content = "content"

	transport = {
		ssh = {
			user = "ubuntu"
			host = "localhost"
			private_key_path = "../fixtures/ssh.pem"
			not_an_arg = "boom"
		}
	}
}`

	k8sCfg := `resource enos_file "bad_k8s" {
	destination = "/tmp/dst"
	content = "content"

	transport = {
		kubernetes = {
            kubeconfig_base64 = "some kubeconfig"
            context_name      = "some context"
            namespace         = "default"
            pod               = "nginx-0"
            container         = "proxy"
			not_an_arg        = "boom"
		}
	}
}`

	nomadCfg := `resource enos_file "bad_nomad" {
	destination = "/tmp/dst"
	content = "content"

	transport = {
		nomad = {
            host = "some host"
            secret_id = "some secret"
            allocation_id = "some allocation id"
            task_name = "some task"
            bogus_arg = "bogus"
		}
	}
}`

	//nolint:paralleltest// because our resource handles it
	for _, test := range []struct {
		name       string
		cfg        string
		errorRegEx *regexp.Regexp
	}{
		{"ssh_transport", sshCfg, regexp.MustCompile(`not_an_arg`)},
		{"k8s_transsport", k8sCfg, regexp.MustCompile(`not_an_arg`)},
		{"nomad_transport", nomadCfg, regexp.MustCompile(`bogus_arg`)},
	} {
		t.Run(test.name, func(tt *testing.T) {
			resource.ParallelTest(tt, resource.TestCase{
				ProtoV6ProviderFactories: testProviders(tt),
				Steps: []resource.TestStep{
					{
						Config:             test.cfg,
						PlanOnly:           true,
						ExpectNonEmptyPlan: false,
						ExpectError:        test.errorRegEx,
					},
				},
			})
		})
	}
}

func TestResourceFileMarshalRoundtrip(t *testing.T) {
	t.Parallel()

	fileState := newFileState()
	ssh := newEmbeddedTransportSSH()
	ssh.Values = testMapPropertiesToStruct([]testProperty{
		{"user", "ubuntu", ssh.User},
		{"host", "localhost", ssh.Host},
		{"private_key", "PRIVATE KEY", ssh.PrivateKey},
		{"private_key_path", "/path/to/key.pem", ssh.PrivateKeyPath},
	})
	k8s := newEmbeddedTransportK8Sv1()
	k8s.Values = testMapPropertiesToStruct([]testProperty{
		{"kubeconfig_base64", "some kubeconfig", k8s.KubeConfigBase64},
		{"context_name", "some context", k8s.ContextName},
		{"namespace", "default", k8s.Namespace},
		{"pod", "nginx-0", k8s.Pod},
		{"container", "proxy", k8s.Container},
	})
	nomad := newEmbeddedTransportNomadv1()
	nomad.Values = testMapPropertiesToStruct([]testProperty{
		{"host", "some host", nomad.Host},
		{"secret_id", "some secret", nomad.SecretID},
		{"allocation_id", "some allocation id", nomad.AllocationID},
		{"task_name", "some task", nomad.TaskName},
	})
	testMapPropertiesToStruct([]testProperty{
		{"id", "foo", fileState.ID},
		{"src", "/tmp/src", fileState.Src},
		{"dst", "/tmp/dst", fileState.Dst},
	})
	assert.NoError(t, fileState.Transport.SetTransportState(ssh, k8s, nomad))

	marshaled, err := state.Marshal(fileState)
	require.NoError(t, err)

	newState := newFileState()
	err = unmarshal(newState, marshaled)
	require.NoError(t, err)

	assert.Equal(t, fileState.ID, newState.ID)
	assert.Equal(t, fileState.Src, newState.Src)
	assert.Equal(t, fileState.Dst, newState.Dst)

	SSH, ok := fileState.Transport.SSH()
	assert.True(t, ok)
	newSSH, ok := newState.Transport.SSH()
	assert.True(t, ok)
	assert.Equal(t, SSH.User, newSSH.User)
	assert.Equal(t, SSH.Host, newSSH.Host)
	assert.Equal(t, SSH.PrivateKey, newSSH.PrivateKey)
	assert.Equal(t, SSH.PrivateKeyPath, newSSH.PrivateKeyPath)

	K8S, ok := fileState.Transport.K8S()
	assert.True(t, ok)
	newK8S, ok := newState.Transport.K8S()
	assert.True(t, ok)
	assert.Equal(t, K8S.KubeConfigBase64, newK8S.KubeConfigBase64)
	assert.Equal(t, K8S.ContextName, newK8S.ContextName)
	assert.Equal(t, K8S.Namespace, newK8S.Namespace)
	assert.Equal(t, K8S.Pod, newK8S.Pod)
	assert.Equal(t, K8S.Container, newK8S.Container)

	nmd, ok := fileState.Transport.Nomad()
	assert.True(t, ok)
	newNmd, ok := newState.Transport.Nomad()
	assert.True(t, ok)
	assert.Equal(t, nmd.Host, newNmd.Host)
	assert.Equal(t, nmd.SecretID, newNmd.SecretID)
	assert.Equal(t, nmd.AllocationID, newNmd.AllocationID)
	assert.Equal(t, nmd.TaskName, newNmd.TaskName)
}

func TestSetProviderConfig(t *testing.T) {
	t.Parallel()

	p := newProviderConfig()
	f := newFile()

	tr := newEmbeddedTransport()
	ssh := newEmbeddedTransportSSH()
	ssh.Values = testMapPropertiesToStruct([]testProperty{
		{"user", "ubuntu", ssh.User},
		{"host", "localhost", ssh.Host},
		{"private_key", "PRIVATE KEY", ssh.PrivateKey},
		{"private_key_path", "/path/to/key.pem", ssh.PrivateKeyPath},
	})
	k8s := newEmbeddedTransportK8Sv1()
	k8s.Values = testMapPropertiesToStruct([]testProperty{
		{"kubeconfig_base64", "some kubeconfig", k8s.KubeConfigBase64},
		{"context_name", "some context", k8s.ContextName},
		{"namespace", "default", k8s.Namespace},
		{"pod", "nginx-0", k8s.Pod},
		{"container", "proxy", k8s.Container},
	})
	nomad := newEmbeddedTransportNomadv1()
	nomad.Values = testMapPropertiesToStruct([]testProperty{
		{"host", "some host", nomad.Host},
		{"secret_id", "some secret", nomad.SecretID},
		{"allocation_id", "some allocation id", nomad.AllocationID},
		{"task_name", "some task", nomad.TaskName},
	})
	assert.NoError(t, p.Transport.SetTransportState(ssh, k8s, nomad))

	require.NoError(t, p.Transport.FromTerraform5Value(tr.Terraform5Value()))
	require.NoError(t, f.SetProviderConfig(p.Terraform5Value()))

	SSH, ok := f.providerConfig.Transport.SSH()
	assert.True(t, ok)
	assert.Equal(t, "ubuntu", SSH.User.Value())
	assert.Equal(t, "localhost", SSH.Host.Value())
	assert.Equal(t, "PRIVATE KEY", SSH.PrivateKey.Value())
	assert.Equal(t, "/path/to/key.pem", SSH.PrivateKeyPath.Value())

	K8S, ok := f.providerConfig.Transport.K8S()
	assert.True(t, ok)
	assert.Equal(t, "some kubeconfig", K8S.KubeConfigBase64.Value())
	assert.Equal(t, "some context", K8S.ContextName.Value())
	assert.Equal(t, "default", K8S.Namespace.Value())
	assert.Equal(t, "nginx-0", K8S.Pod.Value())
	assert.Equal(t, "proxy", K8S.Container.Value())

	nmd, ok := f.providerConfig.Transport.Nomad()
	assert.True(t, ok)
	assert.Equal(t, "some host", nmd.Host.Value())
	assert.Equal(t, "some secret", nmd.SecretID.Value())
	assert.Equal(t, "some allocation id", nmd.AllocationID.Value())
	assert.Equal(t, "some task", nmd.TaskName.Value())
}
