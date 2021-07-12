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
	cfg := template.Must(template.New("enos_vault_start").Parse(`resource "enos_vault_start" "{{.ID}}" {
		{{if .BinPath}}
		bin_path = "{{.BinPath}}"
		{{end}}

		config = {
			api_addr = "{{.Config.APIAddr}}"
			cluster_addr = "{{.Config.ClusterAddr}}"
			listener = {
				type = "{{.Config.Listener.Type}}"
				attributes = {
					{{range $name, $val := .Config.Listener.Attrs}}
					{{$name}} = "{{$val}}"
					{{end}}
				}
			}
			seal = {
				type = "{{.Config.Seal.Type}}"
				attributes = {
					{{range $name, $val := .Config.Seal.Attrs}}
					{{$name}} = "{{$val}}"
					{{end}}
				}
			}
			storage = {
				type = "{{.Config.Storage.Type}}"
				attributes = {
					{{range $name, $val := .Config.Storage.Attrs}}
					{{$name}} = "{{$val}}"
					{{end}}
				}
			}
			ui = true
		}

		{{if .ConfigDir}}
		config_dir = "{{.ConfigDir}}"
		{{end}}

		{{if .License}}
		license = "{{.License}}"
		{{end}}

		{{if .SystemdUnitName}}
		unit_name = "{{.SystemdUnitName}}"
		{{end}}

		{{if .Username}}
		username = "{{.Username}}"
		{{end}}

		transport = {
			ssh = {
				{{if .Transport.SSH.User}}
				user = "{{.Transport.SSH.User}}"
				{{end}}

				{{if .Transport.SSH.Host}}
				host = "{{.Transport.SSH.Host}}"
				{{end}}

				{{if .Transport.SSH.PrivateKey}}
				private_key = <<EOF
{{.Transport.SSH.PrivateKey}}
EOF
				{{end}}

				{{if .Transport.SSH.PrivateKeyPath}}
				private_key_path = "{{.Transport.SSH.PrivateKeyPath}}"
				{{end}}

				{{if .Transport.SSH.Passphrase}}
				passphrase = "{{.Transport.SSH.Passphrase}}"
				{{end}}

				{{if .Transport.SSH.PassphrasePath}}
				passphrase_path = "{{.Transport.SSH.PassphrasePath}}"
				{{end}}
			}
		}
	}`))

	cases := []testAccResourceTemplate{}

	vaultStart := newVaultStartStateV1()
	vaultStart.ID = "foo"
	vaultStart.BinPath = "/opt/vault/bin/vault"
	vaultStart.ConfigDir = "/etc/vault.d"
	vaultStart.Config = &vaultConfig{
		APIAddr:     "http://127.0.0.1:8200",
		ClusterAddr: "http://127.0.0.1:8201",
		Listener: &vaultConfigBlock{
			Type: "tcp",
			Attrs: map[string]interface{}{
				"address":     "0.0.0.0:8200",
				"tls_disable": "true",
			},
		},
		Storage: &vaultConfigBlock{
			Type: "consul",
			Attrs: map[string]interface{}{
				"address": "127.0.0.1:8500",
				"path":    "vault",
			},
		},
		Seal: &vaultConfigBlock{
			Type: "awskms",
			Attrs: map[string]interface{}{
				"kms_key_id": "some-key-id",
			},
		},
	}
	vaultStart.License = "some-license-key"
	vaultStart.SystemdUnitName = "vault"
	vaultStart.Username = "vault"
	vaultStart.Transport.SSH.User = "ubuntu"
	vaultStart.Transport.SSH.Host = "localhost"
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	vaultStart.Transport.SSH.PrivateKey = privateKey
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
				ProtoV5ProviderFactories: testProviders,
				Steps:                    []resource.TestStep{step},
			})
		})
	}
}
