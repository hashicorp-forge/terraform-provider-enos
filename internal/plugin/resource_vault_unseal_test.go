package plugin

import (
	"bytes"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceVaulUnseal tests the vault_unseal resource
func TestAccResourceVaultUnseal(t *testing.T) {
	cfg := template.Must(template.New("enos_vault_unseal").Parse(`resource "enos_vault_unseal" "{{.ID.Value}}" {
		{{if .BinPath.Value}}
		bin_path = "{{.BinPath.Value}}"
		{{end}}

		{{if .VaultAddr.Value}}
		vault_addr = "{{.VaultAddr.Value}}"
		{{end}}

		{{if .SealType.Value}}
		seal_type = "{{.SealType.Value}}"
		{{end}}

		{{if .UnsealKeys.StringValue}}
		unseal_keys = [
		{{range .UnsealKeys.StringValue}}
			"{{.}}",
		{{end}}
		]
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

	vaultUnseal := newVaultUnsealStateV1()
	vaultUnseal.ID.Set("foo")
	vaultUnseal.BinPath.Set("/opt/vault/bin/vault")
	vaultUnseal.VaultAddr.Set("http://127.0.0.1:8200")

	vaultUnseal.SealType.Set("shamir")
	vaultUnseal.UnsealKeys.SetStrings([]string{"bar"})
	vaultUnseal.Transport.SSH.User.Set("ubuntu")
	vaultUnseal.Transport.SSH.Host.Set("localhost")
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	vaultUnseal.Transport.SSH.PrivateKey.Set(privateKey)
	cases = append(cases, testAccResourceTemplate{
		"all fields are loaded correctly",
		vaultUnseal,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "id", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "bin_path", regexp.MustCompile(`^/opt/vault/bin/vault$`)),
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "vault_addr", regexp.MustCompile(`^http://127.0.0.1:8200$`)),
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "seal_type", regexp.MustCompile(`^shamisr$`)),
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "unseal_keys[0]", regexp.MustCompile("^bar$")),
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
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
				ProtoV6ProviderFactories: testProviders(t),
				Steps:                    []resource.TestStep{step},
			})
		})
	}
}
