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

// TestAccResourceVaulUnseal tests the vault_unseal resource.
func TestAccResourceVaultUnseal(t *testing.T) {
	t.Parallel()
	cfg := template.Must(template.New("enos_vault_unseal").
		Funcs(transportRenderFunc).
		Parse(`resource "enos_vault_unseal" "{{.ID.Value}}" {
		{{if .BinPath.Value}}
		bin_path = "{{.BinPath.Value}}"
		{{end}}

		{{if .VaultAddr.Value}}
		vault_addr = "{{.VaultAddr.Value}}"
		{{end}}

		{{if .SystemdUnitName.Value}}
		unit_name = "{{.SystemdUnitName.Value}}"
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

		{{ renderTransport .Transport }}
	}`))

	cases := []testAccResourceTemplate{}

	vaultUnseal := newVaultUnsealStateV1()
	vaultUnseal.ID.Set("foo")
	vaultUnseal.BinPath.Set("/opt/vault/bin/vault")
	vaultUnseal.VaultAddr.Set("http://127.0.0.1:8200")
	vaultUnseal.SystemdUnitName.Set("vaulter")
	vaultUnseal.SealType.Set("shamir")
	vaultUnseal.UnsealKeys.SetStrings([]string{"bar"})
	ssh := newEmbeddedTransportSSH()
	ssh.User.Set("ubuntu")
	ssh.Host.Set("localhost")
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	ssh.PrivateKey.Set(privateKey)
	assert.NoError(t, vaultUnseal.Transport.SetTransportState(ssh))
	cases = append(cases, testAccResourceTemplate{
		"all fields are loaded correctly",
		vaultUnseal,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "id", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "bin_path", regexp.MustCompile(`^/opt/vault/bin/vault$`)),
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "vault_addr", regexp.MustCompile(`^http://127.0.0.1:8200$`)),
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "unit_name", regexp.MustCompile(`^vaulter$`)),
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "seal_type", regexp.MustCompile(`^shamisr$`)),
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "unseal_keys[0]", regexp.MustCompile("^bar$")),
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_vault_unseal.foo", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
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
