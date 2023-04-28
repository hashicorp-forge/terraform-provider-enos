package plugin

import (
	"bytes"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceConsulStart tests the consul_start resource.
func TestAccResourceConsulStart(t *testing.T) {
	t.Parallel()

	cfg := template.Must(template.New("enos_consul_start").
		Funcs(transportRenderFunc).
		Parse(`resource "enos_consul_start" "{{.ID.Value}}" {
		{{if .BinPath.Value}}
		bin_path = "{{.BinPath.Value}}"
		{{end}}

		config = {
			datacenter = "{{.Config.Datacenter.Value}}"
			data_dir = "{{.Config.DataDir.Value}}"
			retry_join = [{{range .Config.RetryJoin.Value -}}
			"{{.}}",
			{{end -}}
			]
			server = true
			bootstrap_expect = {{.Config.BootstrapExpect.Value}}
			log_file = "{{.Config.LogFile.Value}}"
			log_level = "{{.Config.LogLevel.Value}}"
		}

		{{if .ConfigDir.Value}}
		config_dir = "{{.ConfigDir.Value}}"
		{{end}}

		{{if .DataDir.Value}}
		data_dir = "{{.DataDir.Value}}"
		{{end}}

		{{if .License.Value}}
		license = "{{.License.Value}}"
		{{end}}

		{{if .SystemdUnitName.Value}}
		unit_name = "{{.SystemdUnitName.Value}}"
		{{end}}

		{{if .Username.Value}}
		username = "{{.Username.Value}}"
		{{end}}

		{{renderTransport .Transport}}
	}`))

	cases := []testAccResourceTemplate{}

	consulStart := newConsulStartStateV1()
	consulStart.ID.Set("foo")
	consulStart.BinPath.Set("/opt/consul/bin/consul")
	consulStart.ConfigDir.Set("/etc/consul.d")
	consulStart.Config.Datacenter.Set("dc1")
	consulStart.Config.DataDir.Set("/opt/consul/data")
	consulStart.Config.RetryJoin.SetStrings([]string{"provider=aws tag_key=Type tag_value=dc1"})
	consulStart.Config.Server.Set(true)
	consulStart.Config.BootstrapExpect.Set(3)
	consulStart.Config.LogFile.Set("/var/log")
	consulStart.Config.LogLevel.Set("INFO")
	consulStart.License.Set("some-license-key")
	consulStart.SystemdUnitName.Set("consul")
	consulStart.Username.Set("consul")
	ssh := newEmbeddedTransportSSH()
	ssh.User.Set("ubuntu")
	ssh.Host.Set("localhost")
	assert.NoError(t, consulStart.Transport.SetTransportState(ssh))
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	ssh, ok := consulStart.Transport.SSH()
	assert.True(t, ok)
	ssh.PrivateKey.Set(privateKey)
	cases = append(cases, testAccResourceTemplate{
		"all fields are loaded correctly",
		consulStart,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_consul_start.foo", "id", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "bin_path", regexp.MustCompile(`^/opt/consul/bin/consul$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "config_dir", regexp.MustCompile(`^/etc/consul.d$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "config.datacenter", regexp.MustCompile(`dc1$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "config.data_dir", regexp.MustCompile(`^/opt/consul/data$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "config.retry_join", regexp.MustCompile(`^provider^$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "config.server", regexp.MustCompile(`^true$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "config.bootstrap_expect", regexp.MustCompile(`^3$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "config.log_file", regexp.MustCompile(`^/var/log$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "config.log_level", regexp.MustCompile(`^INFO$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "config.license", regexp.MustCompile(`^some-license-key$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "config.unit_name", regexp.MustCompile(`^consul$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "config.username", regexp.MustCompile(`^consul$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_consul_start.foo", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
		),
		false,
	})

	//nolint:paralleltest// because our resource handles it
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
