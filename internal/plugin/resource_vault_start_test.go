package plugin

import (
	"bytes"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceVaultStart tests the vault_start resource
func TestAccResourceVaultStart(t *testing.T) {
	cfg := template.Must(template.New("enos_vault_start").Parse(`resource "enos_vault_start" "{{.ID.Value}}" {
		{{if .BinPath.Value}}
		bin_path = "{{.BinPath.Value}}"
		{{end}}

		config = {
			api_addr = "{{.Config.APIAddr.Value}}"
			cluster_addr = "{{.Config.ClusterAddr.Value}}"
			listener = {
				type = "{{.Config.Listener.Type.Value}}"
				attributes = {
					{{range $name, $val := .Config.Listener.Attrs.Value}}
					{{$name}} = "{{$val}}"
					{{end}}
				}
			}
			seal = {
				type = "{{.Config.Seal.Type.Value}}"
				attributes = {
					{{range $name, $val := .Config.Seal.Attrs.Value}}
					{{$name}} = "{{$val}}"
					{{end}}
				}
			}
			storage = {
				type = "{{.Config.Storage.Type.Value}}"
				attributes = {
					{{range $name, $val := .Config.Storage.Attrs.Value}}
					{{$name}} = "{{$val}}"
					{{end}}
				}
			}
			ui = true
		}

		{{if .ConfigDir.Value}}
		config_dir = "{{.ConfigDir.Value}}"
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

	vaultStart := newVaultStartStateV1()
	vaultStart.ID.Set("foo")
	vaultStart.BinPath.Set("/opt/vault/bin/vault")
	vaultStart.ConfigDir.Set("/etc/vault.d")
	vaultStart.Config.APIAddr.Set("http://127.0.0.1:8200")
	vaultStart.Config.ClusterAddr.Set("http://127.0.0.1:8201")
	vaultStart.Config.Listener.Type.Set("tcp")
	vaultStart.Config.Listener.Attrs.Set(map[string]interface{}{
		"address":     "0.0.0.0:8200",
		"tls_disable": "true",
	})
	vaultStart.Config.Storage.Type.Set("consul")
	vaultStart.Config.Storage.Attrs.Set(map[string]interface{}{
		"address": "127.0.0.1:8500",
		"path":    "vault",
	})
	vaultStart.Config.Seal.Type.Set("awskms")
	vaultStart.Config.Seal.Attrs.Set(map[string]interface{}{
		"kms_key_id": "some-key-id",
	})
	vaultStart.License.Set("some-license-key")
	vaultStart.SystemdUnitName.Set("vault")
	vaultStart.Username.Set("vault")
	vaultStart.Transport.SSH.User.Set("ubuntu")
	vaultStart.Transport.SSH.Host.Set("localhost")
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	vaultStart.Transport.SSH.PrivateKey.Set(privateKey)
	cases = append(cases, testAccResourceTemplate{
		"all fields are loaded correctly",
		vaultStart,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_vault_start.foo", "id", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "bin_path", regexp.MustCompile(`^/opt/vault/bin/vault$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config_dir", regexp.MustCompile(`^/etc/vault.d$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.api_addr", regexp.MustCompile(`^http://127.0.0.1:8200$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.cluster_addr", regexp.MustCompile(`^http://127.0.0.1:8201$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.listener.type", regexp.MustCompile(`^tcp$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.listener.attributes.address", regexp.MustCompile(`^0.0.0.0:8200$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.listener.attributes.tls_disable", regexp.MustCompile(`^true$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.storage.type", regexp.MustCompile(`^consul$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.storage.attributes.address", regexp.MustCompile(`^127.0.0.0:8500$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.storage.attributes.path", regexp.MustCompile(`^vault$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.seal.type", regexp.MustCompile(`^awskms$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.seal.attributes.kms_key_id", regexp.MustCompile(`^some-key-id$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.license", regexp.MustCompile(`^some-license-key$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.unit_name", regexp.MustCompile(`^vault$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.username", regexp.MustCompile(`^vault$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
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
