package plugin

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"testing"
	"text/template"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testAccResourceTemplate struct {
	name  string
	state State
	check resource.TestCheckFunc
	apply bool
}

type testAccResourceTransportTemplate struct {
	name             string
	state            State
	check            resource.TestCheckFunc
	transport        *embeddedTransportV1
	resourceTemplate *template.Template
	transportUsed    string
}

// TestAccResourceFileResourceTransport tests both the basic enos_file resource interface
// but also the embedded transport interface. As the embedded transport isn't
// an actual resource we're doing it here.
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

	sshResourceTransport := template.Must(template.New("enos_file").Parse(`resource "enos_file" "{{.ID.Value}}" {
		{{if .Src.Value}}
		source = "{{.Src.Value}}"
		{{end}}

		{{if .Content.Value}}
		content = <<EOF
{{.Content.Value}}"
EOF
		{{end}}

		destination = "{{.Dst.Value}}"

		transport = {
			ssh = {
				user = "{{.Transport.SSH.User.Value}}"
				host = "{{.Transport.SSH.Host.Value}}"

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

	k8sResourceTransport := template.Must(template.New("enos_file").Parse(`resource "enos_file" "{{.ID.Value}}" {
		{{if .Src.Value}}
		source = "{{.Src.Value}}"
		{{end}}

		{{if .Content.Value}}
		content = <<EOF
{{.Content.Value}}"
EOF
		{{end}}

		destination = "{{.Dst.Value}}"

		transport = {
			kubernetes = {
				kubeconfig   = "{{.Transport.K8S.KubeConfig.Value}}"
				context_name = "{{.Transport.K8S.ContextName.Value}}"
				pod          = "{{.Transport.K8S.Pod.Value}}"

				{{if .Transport.K8S.Namespace.Value}}
				namespace = "{{.Transport.K8S.Namespace.Value}}"
				{{end}}

				{{if .Transport.K8S.Container.Value}}
				container = "{{.Transport.K8S.Container.Value}}"
				{{end}}
			}
		}
	}`))

	cases := []testAccResourceTransportTemplate{}

	keyNoPass := newFileState()
	keyNoPass.ID.Set("foo")
	keyNoPass.Src.Set("../fixtures/src.txt")
	keyNoPass.Dst.Set("/tmp/dst")
	keyNoPass.Transport.SSH.User.Set("ubuntu")
	keyNoPass.Transport.SSH.Host.Set("localhost")
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	keyNoPass.Transport.SSH.PrivateKey.Set(privateKey)
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
		sshResourceTransport,
		"ssh",
	})

	keyPathNoPass := newFileState()
	keyPathNoPass.ID.Set("foo")
	keyPathNoPass.Src.Set("../fixtures/src.txt")
	keyPathNoPass.Dst.Set("/tmp/dst")
	keyPathNoPass.Transport.SSH.User.Set("ubuntu")
	keyPathNoPass.Transport.SSH.Host.Set("localhost")
	keyPathNoPass.Transport.SSH.PrivateKeyPath.Set("../fixtures/ssh.pem")
	cases = append(cases, testAccResourceTransportTemplate{
		"[ssh] private key from a file path with no passphrase",
		keyPathNoPass,
		resource.ComposeTestCheckFunc(),
		keyPathNoPass.Transport,
		sshResourceTransport,
		"ssh",
	})

	keyPass := newFileState()
	keyPass.ID.Set("foo")
	keyPass.Src.Set("../fixtures/src.txt")
	keyPass.Dst.Set("/tmp/dst")
	keyPass.Transport.SSH.User.Set("ubuntu")
	keyPass.Transport.SSH.Host.Set("localhost")
	keyPass.Transport.SSH.PrivateKeyPath.Set("../fixtures/ssh_pass.pem")
	passphrase, err := readTestFile("../fixtures/passphrase.txt")
	require.NoError(t, err)
	keyPass.Transport.SSH.Passphrase.Set(passphrase)
	cases = append(cases, testAccResourceTransportTemplate{
		"[ssh] private key value with passphrase value",
		keyPass,
		resource.ComposeTestCheckFunc(),
		keyPass.Transport,
		sshResourceTransport,
		"ssh",
	})

	keyPassPath := newFileState()
	keyPassPath.ID.Set("foo")
	keyPassPath.Src.Set("../fixtures/src.txt")
	keyPassPath.Dst.Set("/tmp/dst")
	keyPassPath.Transport.SSH.User.Set("ubuntu")
	keyPassPath.Transport.SSH.Host.Set("localhost")
	keyPassPath.Transport.SSH.PrivateKeyPath.Set("../fixtures/ssh_pass.pem")
	keyPassPath.Transport.SSH.PassphrasePath.Set("../fixtures/passphrase.txt")
	cases = append(cases, testAccResourceTransportTemplate{
		"[ssh] private key value with passphrase from file path",
		keyPassPath,
		resource.ComposeTestCheckFunc(),
		keyPassPath.Transport,
		sshResourceTransport,
		"ssh",
	})

	content := newFileState()
	content.ID.Set("foo")
	content.Content.Set("hello world")
	content.Dst.Set("/tmp/dst")
	content.Transport.SSH.User.Set("ubuntu")
	content.Transport.SSH.Host.Set("localhost")
	content.Transport.SSH.PrivateKeyPath.Set("../fixtures/ssh_pass.pem")
	content.Transport.SSH.PassphrasePath.Set("../fixtures/passphrase.txt")
	cases = append(cases, testAccResourceTransportTemplate{
		"[ssh] with string content instead of source file",
		content,
		resource.ComposeTestCheckFunc(),
		content.Transport,
		sshResourceTransport,
		"ssh",
	})

	content = newFileState()
	content.ID.Set("foo")
	content.Content.Set("hello world")
	content.Dst.Set("/tmp/dst")
	content.Transport.K8S.KubeConfig.Set("../fixtures/kubeconfig")
	content.Transport.K8S.ContextName.Set("kind-kind")
	content.Transport.K8S.Pod.Set("some-pod")
	cases = append(cases, testAccResourceTransportTemplate{
		"[kubernetes] with string content instead of source file",
		content,
		resource.ComposeTestCheckFunc(),
		content.Transport,
		k8sResourceTransport,
		"k8s",
	})

	for _, test := range cases {
		// Run them with resource defined transport config
		t.Run(fmt.Sprintf("resource transport %s", test.name), func(t *testing.T) {
			unsetAllEnosEnv(t)
			defer resetEnv(t)

			buf := bytes.Buffer{}
			err := sshResourceTransport.Execute(&buf, test.state)
			if err != nil {
				t.Fatalf("error executing test template: %s", err.Error())
			}

			step := resource.TestStep{
				Config:             buf.String(),
				Check:              test.check,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			}

			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testProviders(t),
				Steps:                    []resource.TestStep{step},
			})
		})

		// Run them with provider config passed through the environment
		t.Run(fmt.Sprintf("provider transport %s", test.name), func(t *testing.T) {
			unsetAllEnosEnv(t)
			switch test.transportUsed {
			case "ssh":
				setEnosSSHEnv(t, test.transport)
			case "k8s":
				setEnosK8SEnv(t, test.transport)
			default:
				t.Errorf("unknown transport type: %s", test.transportUsed)
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
		realTestSrc.Transport.SSH.User.Set(os.Getenv("ENOS_TRANSPORT_USER"))
		realTestSrc.Transport.SSH.Host.Set(host)
		realTestSrc.Transport.SSH.PrivateKeyPath.Set(os.Getenv("ENOS_TRANSPORT_PRIVATE_KEY_PATH"))
		realTestSrc.Transport.SSH.PassphrasePath.Set(os.Getenv("ENOS_TRANSPORT_PASSPHRASE_PATH"))
		cases = append(cases, testAccResourceTransportTemplate{
			"[ssh] real test source file",
			realTestSrc,
			resource.ComposeTestCheckFunc(),
			realTestSrc.Transport,
			sshResourceTransport,
			"ssh",
		})

		realTestContent := newFileState()
		realTestContent.ID.Set("real")
		realTestContent.Content.Set("string")
		realTestContent.Dst.Set("/tmp/real_test_content")
		realTestContent.Transport.SSH.User.Set(os.Getenv("ENOS_TRANSPORT_USER"))
		realTestContent.Transport.SSH.Host.Set(host)
		realTestContent.Transport.SSH.PrivateKeyPath.Set(os.Getenv("ENOS_TRANSPORT_PRIVATE_KEY_PATH"))
		realTestContent.Transport.SSH.PassphrasePath.Set(os.Getenv("ENOS_TRANSPORT_PASSPHRASE_PATH"))
		cases = append(cases, testAccResourceTransportTemplate{
			"[ssh] real test content",
			realTestContent,
			resource.ComposeTestCheckFunc(),
			realTestContent.Transport,
			sshResourceTransport,
			"ssh",
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
// psuedo type we cannot rely on Terraform's built-in validation.
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

	k8sCfg := `resource enos_file "bad_ssh" {
	destination = "/tmp/dst"
	content = "content"

	transport = {
		kubernetes = {
            kubeconfig   = "some kubeconfig"
            context_name = "some context"
            namespace = "default"
            pod = "nginx-0"
            container = "proxy"
			not_an_arg = "boom"
		}
	}
}`

	for _, test := range []struct {
		name       string
		cfg        string
		errorRegEx *regexp.Regexp
	}{
		{"ssh_transport", sshCfg, regexp.MustCompile(`not_an_arg`)},
		{"k8s_transsport", k8sCfg, regexp.MustCompile(`not_an_arg`)},
	} {
		t.Run(test.name, func(tt *testing.T) {
			resource.Test(tt, resource.TestCase{
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
	state := newFileState()
	state.Transport.SSH.Values = testMapPropertiesToStruct([]testProperty{
		{"user", "ubuntu", state.Transport.SSH.User},
		{"host", "localhost", state.Transport.SSH.Host},
		{"private_key", "PRIVATE KEY", state.Transport.SSH.PrivateKey},
		{"private_key_path", "/path/to/key.pem", state.Transport.SSH.PrivateKeyPath},
	})
	state.Transport.K8S.Values = testMapPropertiesToStruct([]testProperty{
		{"kubeconfig", "some kubeconfig", state.Transport.K8S.KubeConfig},
		{"context_name", "some context", state.Transport.K8S.ContextName},
		{"namespace", "default", state.Transport.K8S.Namespace},
		{"pod", "nginx-0", state.Transport.K8S.Pod},
		{"container", "proxy", state.Transport.K8S.Container},
	})
	testMapPropertiesToStruct([]testProperty{
		{"id", "foo", state.ID},
		{"src", "/tmp/src", state.Src},
		{"dst", "/tmp/dst", state.Dst},
	})

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
	assert.Equal(t, state.Transport.K8S.KubeConfig, newState.Transport.K8S.KubeConfig)
	assert.Equal(t, state.Transport.K8S.ContextName, newState.Transport.K8S.ContextName)
	assert.Equal(t, state.Transport.K8S.Namespace, newState.Transport.K8S.Namespace)
	assert.Equal(t, state.Transport.K8S.Pod, newState.Transport.K8S.Pod)
	assert.Equal(t, state.Transport.K8S.Container, newState.Transport.K8S.Container)
}

func TestSetProviderConfig(t *testing.T) {
	p := newProviderConfig()
	f := newFile()

	tr := newEmbeddedTransport()
	tr.SSH.Values = testMapPropertiesToStruct([]testProperty{
		{"user", "ubuntu", tr.SSH.User},
		{"host", "localhost", tr.SSH.Host},
		{"private_key", "PRIVATE KEY", tr.SSH.PrivateKey},
		{"private_key_path", "/path/to/key.pem", tr.SSH.PrivateKeyPath},
	})
	tr.K8S.Values = testMapPropertiesToStruct([]testProperty{
		{"kubeconfig", "some kubeconfig", tr.K8S.KubeConfig},
		{"context_name", "some context", tr.K8S.ContextName},
		{"namespace", "default", tr.K8S.Namespace},
		{"pod", "nginx-0", tr.K8S.Pod},
		{"container", "proxy", tr.K8S.Container},
	})

	require.NoError(t, p.Transport.FromTerraform5Value(tr.Terraform5Value()))
	require.NoError(t, f.SetProviderConfig(p.Terraform5Value()))

	assert.Equal(t, "ubuntu", f.providerConfig.Transport.SSH.User.Value())
	assert.Equal(t, "localhost", f.providerConfig.Transport.SSH.Host.Value())
	assert.Equal(t, "PRIVATE KEY", f.providerConfig.Transport.SSH.PrivateKey.Value())
	assert.Equal(t, "/path/to/key.pem", f.providerConfig.Transport.SSH.PrivateKeyPath.Value())
	assert.Equal(t, "some kubeconfig", f.providerConfig.Transport.K8S.KubeConfig.Value())
	assert.Equal(t, "some context", f.providerConfig.Transport.K8S.ContextName.Value())
	assert.Equal(t, "default", f.providerConfig.Transport.K8S.Namespace.Value())
	assert.Equal(t, "nginx-0", f.providerConfig.Transport.K8S.Pod.Value())
	assert.Equal(t, "proxy", f.providerConfig.Transport.K8S.Container.Value())
}
