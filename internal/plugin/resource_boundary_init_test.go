package plugin

import (
	"bytes"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceBoundaryInit tests the boundary_init resource.
func TestAccResourceBoundaryInit(t *testing.T) {
	t.Parallel()

	cfg := template.Must(template.New("enos_boundary_init").
		Funcs(transportRenderFunc).
		Parse(`resource "enos_boundary_init" "{{.ID.Value}}" {
  {{if .BinName.Value}}
  bin_name = "{{.BinName.Value}}"
  {{end}}

  {{if .BinPath.Value}}
  bin_path = "{{.BinPath.Value}}"
  {{end}}

  {{if .ConfigPath.Value}}
  config_path = "{{.ConfigPath.Value}}"
  {{end}}

  {{ renderTransport .Transport }}
}`))

	cases := []testAccResourceTemplate{}

	boundaryStart := newBoundaryInitStateV1()
	boundaryStart.ID.Set("foo")
	boundaryStart.BinName.Set("boundary-worker")
	boundaryStart.BinPath.Set("/opt/boundary/bin")
	boundaryStart.ConfigPath.Set("/etc/boundary")
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	ssh := newEmbeddedTransportSSH()
	ssh.PrivateKey.Set(privateKey)
	require.NoError(t, boundaryStart.Transport.SetTransportState(ssh))
	cases = append(cases, testAccResourceTemplate{
		"all fields are loaded correctly",
		boundaryStart,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_boundary_init.foo", "id", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_boundary_init.foo", "bin_name", regexp.MustCompile(`^boundary-worker$`)),
			resource.TestMatchResourceAttr("enos_boundary_init.foo", "bin_path", regexp.MustCompile(`^/opt/boundary/bin$`)),
			resource.TestMatchResourceAttr("enos_boundary_init.foo", "config_path", regexp.MustCompile(`^/etc/boundary$`)),
		),
		false,
	})

	//nolint:paralleltest// because our resource handles it
	for _, test := range cases {
		test := test
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
