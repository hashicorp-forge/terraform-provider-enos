package plugin

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

type testAccResourceTemplate struct {
	name  string
	state State
	check resource.TestCheckFunc
	apply bool
}

type testAccResourceTransportTemplate struct {
	name      string
	state     State
	check     resource.TestCheckFunc
	transport *embeddedTransportV1
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

	resourceTransport := template.Must(template.New("enos_file").Parse(`resource "enos_file" "{{.ID.Value}}" {
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
		"private key value with no passphrase",
		keyNoPass,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_file.foo", "id", regexp.MustCompile(`^static$`)),
			resource.TestMatchResourceAttr("enos_file.foo", "source", regexp.MustCompile(`^/tmp/foo$`)),
			resource.TestMatchResourceAttr("enos_file.foo", "destination", regexp.MustCompile(`^/tmp/foo$`)),
			resource.TestMatchResourceAttr("enos_file.foo", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_file.foo", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
		),
		keyNoPass.Transport,
	})

	keyPathNoPass := newFileState()
	keyPathNoPass.ID.Set("foo")
	keyPathNoPass.Src.Set("../fixtures/src.txt")
	keyPathNoPass.Dst.Set("/tmp/dst")
	keyPathNoPass.Transport.SSH.User.Set("ubuntu")
	keyPathNoPass.Transport.SSH.Host.Set("localhost")
	keyPathNoPass.Transport.SSH.PrivateKeyPath.Set("../fixtures/ssh.pem")
	cases = append(cases, testAccResourceTransportTemplate{
		"private key from a file path with no passphrase",
		keyPathNoPass,
		resource.ComposeTestCheckFunc(),
		keyPathNoPass.Transport,
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
		"private key value with passphrase value",
		keyPass,
		resource.ComposeTestCheckFunc(),
		keyPass.Transport,
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
		"private key value with passphrase from file path",
		keyPassPath,
		resource.ComposeTestCheckFunc(),
		keyPassPath.Transport,
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
		"with string content instead of source file",
		content,
		resource.ComposeTestCheckFunc(),
		content.Transport,
	})

	for _, test := range cases {
		// Run them with resource defined transport config
		t.Run(fmt.Sprintf("resource transport %s", test.name), func(t *testing.T) {
			unsetEnosEnv(t)
			defer resetEnv(t)

			buf := bytes.Buffer{}
			err := resourceTransport.Execute(&buf, test.state)
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
				ProtoV5ProviderFactories: testProviders,
				Steps:                    []resource.TestStep{step},
			})
		})

		// Run them with provider config passed through the environment
		t.Run(fmt.Sprintf("provider transport %s", test.name), func(t *testing.T) {
			unsetEnosEnv(t)
			setEnosEnv(t, test.transport)
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
				ProtoV5ProviderFactories: testProviders,
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
			"real test source file",
			realTestSrc,
			resource.ComposeTestCheckFunc(),
			realTestSrc.Transport,
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
			"real test content",
			realTestContent,
			resource.ComposeTestCheckFunc(),
			realTestContent.Transport,
		})

		for _, test := range cases {
			// Run them with resource defined transport config
			t.Run(fmt.Sprintf("resource transport %s", test.name), func(t *testing.T) {
				defer resetEnv(t)

				buf := bytes.Buffer{}
				err := resourceTransport.Execute(&buf, test.state)
				if err != nil {
					t.Fatalf("error executing test template: %s", err.Error())
				}

				step := resource.TestStep{
					Config:             buf.String(),
					Check:              test.check,
					PlanOnly:           false,
					ExpectNonEmptyPlan: false,
				}

				resource.Test(t, resource.TestCase{
					ProtoV5ProviderFactories: testProviders,
					Steps:                    []resource.TestStep{step},
				})
			})

			// Run them with provider config passed through the environment
			t.Run(fmt.Sprintf("provider transport %s", test.name), func(t *testing.T) {
				resetEnv(t)

				buf := bytes.Buffer{}
				err := providerTransport.Execute(&buf, test.state)
				if err != nil {
					t.Fatalf("error executing test template: %s", err.Error())
				}

				step := resource.TestStep{
					Config:             buf.String(),
					Check:              test.check,
					PlanOnly:           false,
					ExpectNonEmptyPlan: false,
				}

				resource.Test(t, resource.TestCase{
					ProtoV5ProviderFactories: testProviders,
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
	cfg := `resource enos_file "bad_ssh" {
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
	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testProviders,
		Steps: []resource.TestStep{
			{
				Config:             cfg,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				ExpectError:        regexp.MustCompile(`not_an_arg`),
			},
		},
	})
}

func TestResourceFileMarshalRoundtrip(t *testing.T) {
	state := newFileState()
	state.Transport.SSH.Values = testMapPropertiesToStruct([]testProperty{
		{"user", "ubuntu", state.Transport.SSH.User},
		{"host", "localhost", state.Transport.SSH.Host},
		{"private_key", "PRIVATE KEY", state.Transport.SSH.PrivateKey},
		{"private_key_path", "/path/to/key.pem", state.Transport.SSH.PrivateKeyPath},
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

	require.NoError(t, p.Transport.FromTerraform5Value(tr.Terraform5Value()))
	require.NoError(t, f.SetProviderConfig(p.Terraform5Value()))

	assert.Equal(t, "ubuntu", f.providerConfig.Transport.SSH.User.Value())
	assert.Equal(t, "localhost", f.providerConfig.Transport.SSH.Host.Value())
	assert.Equal(t, "PRIVATE KEY", f.providerConfig.Transport.SSH.PrivateKey.Value())
	assert.Equal(t, "/path/to/key.pem", f.providerConfig.Transport.SSH.PrivateKeyPath.Value())
}
