package plugin

import (
	"bytes"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceVaultInit tests the vault_init resource
func TestAccResourceVaultInit(t *testing.T) {
	cfg := template.Must(template.New("enos_vault_init").Parse(`resource "enos_vault_init" "{{.ID.Value}}" {
		{{if .BinPath.Value}}
		bin_path = "{{.BinPath.Value}}"
		{{end}}

		{{if .VaultAddr.Value}}
		vault_addr = "{{.VaultAddr.Value}}"
		{{end}}

		{{if .KeyShares.Value}}
		key_shares = {{.KeyShares.Value}}
		{{end}}

		{{if .KeyThreshold.Value}}
		key_threshold = {{.KeyThreshold.Value}}
		{{end}}

		{{if .PGPKeys.StringValue}}
		pgp_keys = [ {{ range $i, $key := .PGPKeys.StringValue}} {{if $i}}, {{end}}"{{$key}}" {{end}} ]
		{{end}}

		{{if .RecoveryShares.Value}}
		recovery_shares = {{.RecoveryShares.Value}}
		{{end}}

		{{if .RecoveryThreshold.Value}}
		recovery_threshold = {{.RecoveryThreshold.Value}}
		{{end}}

		{{if .RecoveryPGPKeys.StringValue}}
		recovery_pgp_keys = [ {{ range $i, $key := .RecoveryPGPKeys.StringValue}} {{if $i}}, {{end}}"{{$key}}" {{end}} ]
		{{end}}

		{{if .RootTokenPGPKey.Value}}
		root_token_pgp_key = "{{.RootTokenPGPKey.Value}}"
		{{end}}

		{{if .ConsulAuto.Value}}
		consul_auto = true
		{{end}}

		{{if .ConsulService.Value}}
		consul_service = "{{.ConsulService.Value}}"
		{{end}}

		{{if .StoredShares.Value}}
		stored_shares = {{.StoredShares.Value}}
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

	vaultInit := newVaultInitStateV1()
	vaultInit.ID.Set("foo")
	vaultInit.BinPath.Set("/opt/vault/bin/vault")
	vaultInit.VaultAddr.Set("http://127.0.0.1:8200")
	vaultInit.KeyShares.Set(7)
	vaultInit.KeyThreshold.Set(5)
	vaultInit.PGPKeys.SetStrings([]string{"keybase:foo", "keybase:bar"})
	vaultInit.RecoveryShares.Set(6)
	vaultInit.RecoveryThreshold.Set(4)
	vaultInit.RecoveryPGPKeys.SetStrings([]string{"keybase:bar", "keybase:baz"})
	vaultInit.RootTokenPGPKey.Set("keybase:hashicorp")
	vaultInit.ConsulAuto.Set(true)
	vaultInit.ConsulService.Set("vault")
	vaultInit.StoredShares.Set(7)
	vaultInit.Transport.SSH.User.Set("ubuntu")
	vaultInit.Transport.SSH.Host.Set("localhost")
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	vaultInit.Transport.SSH.PrivateKey.Set(privateKey)
	cases = append(cases, testAccResourceTemplate{
		"all fields are loaded correctly",
		vaultInit,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_vault_start.foo", "id", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "bin_path", regexp.MustCompile(`^/opt/vault/bin/vault$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "vault_addr", regexp.MustCompile(`^http://127.0.0.1:8200$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "key_shares", regexp.MustCompile(`^7$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "key_threshold", regexp.MustCompile(`^5$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "pgp_keys[0]", regexp.MustCompile(`^keybase:foo$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "recovery_shares", regexp.MustCompile(`^6$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "recovery_threshold", regexp.MustCompile(`^4$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "root_token_pgp_key", regexp.MustCompile(`^keybase:hashicorp$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "consul_auto", regexp.MustCompile(`^true$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "consul_service", regexp.MustCompile(`^vault$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "stored_shares", regexp.MustCompile(`^7$`)),
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
