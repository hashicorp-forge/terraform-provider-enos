package plugin

import (
	"bytes"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceBoundaryStart tests the boundary_start resource
func TestAccResourceBoundaryStart(t *testing.T) {
	cfg := template.Must(template.New("enos_boundary_start").Parse(`resource "enos_boundary_start" "{{.ID.Value}}" {
		{{if .BinPath.Value}}
		bin_path = "{{.BinPath.Value}}"
		{{end}}

		{{if .ConfigPath.Value}}
		config_path = "{{.ConfigPath.Value}}"
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

	boundaryStart := newBoundaryStartStateV1()
	boundaryStart.ID.Set("foo")
	boundaryStart.BinPath.Set("/opt/boundary/bin")
	boundaryStart.ConfigPath.Set("/etc/boundary")
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	boundaryStart.Transport.SSH.PrivateKey.Set(privateKey)
	cases = append(cases, testAccResourceTemplate{
		"all fields are loaded correctly",
		boundaryStart,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_boundary_start.foo", "id", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_boundary_start.foo", "bin_path", regexp.MustCompile(`^/opt/boundary/bin$`)),
			resource.TestMatchResourceAttr("enos_boundary_start.foo", "config_path", regexp.MustCompile(`^/etc/boundary$`)),
		),
		false,
	})

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
				ProtoV6ProviderFactories: testProviders,
				Steps:                    []resource.TestStep{step},
			})
		})
	}
}
