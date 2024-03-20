// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"bytes"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestVaultStartConfigOptionalAttrs tests that we can set tftypes that have optional attributes
// and also that Seals can support different types.
func TestVaultStartConfigOptionalAttrs(t *testing.T) {
	t.Parallel()

	vaultCfg := newVaultConfig()
	vaultCfg.APIAddr.Set("http://127.0.0.1:8200")
	vaultCfg.ClusterAddr.Set("http://127.0.0.1:8201")
	vaultCfg.ClusterName.Set("avaultcluster")
	vaultCfg.Listener.Set(newVaultConfigBlockSet(
		"tcp", map[string]any{
			"address":     "0.0.0.0:8200",
			"tls_disable": "true",
		}, "config", "listener"))
	vaultCfg.LogLevel.Set("debug")
	vaultCfg.Storage.Set(newVaultConfigBlockSet("consul", map[string]any{
		"address": "127.0.0.1:8500",
		"path":    "vault",
	}, "config", "storage"))
	vaultCfg.Seal.Set(newVaultConfigBlockSet("awskms", map[string]any{
		"kms_key_id": "some-key-id",
	}, "config", "seal"))
	require.NoError(t, vaultCfg.Seals.SetSeals(map[string]*vaultConfigBlockSet{
		"primary": newVaultConfigBlockSet("awskms", map[string]any{
			"kms_key_id": "some-key-id",
		}, "config", "seal"),
		"secondary": newVaultConfigBlockSet("pkcs11", map[string]any{
			"lib":            "/usr/lib/softhsm/libsofthsm2.so",
			"slot":           "730906792",
			"pin":            "1234",
			"key_label":      "hsm:v1:vault",
			"hmac_key_label": "hsm:v1:vault-hmac",
			"generate_key":   "true",
			"priority":       "1",
			"key_name":       "hsm1",
		}, "config", "seal"),
		"tertiary": newVaultConfigBlockSet("none", nil, "config", "seal"),
	}))

	// Make sure we can create a dynamic value with optional attrs
	val := vaultCfg.Terraform5Value()
	_, err := tfprotov6.NewDynamicValue(val.Type(), val)
	require.NoError(t, err)

	// Make sure we can create a new tftypes.Value from our seals
	_ = vaultCfg.Seals.Terraform5Value()
}

// TestAccResourceVaultStart tests the vault_start resource.
func TestAccResourceVaultStart(t *testing.T) {
	t.Parallel()
	cfg := template.Must(template.New("enos_vault_start").
		Funcs(transportRenderFunc).
		Parse(`resource "enos_vault_start" "{{.ID.Value}}" {
		{{if .BinPath.Value}}
		bin_path = "{{.BinPath.Value}}"
		{{end}}

		config = {
			api_addr = "{{.Config.APIAddr.Value}}"
			cluster_addr = "{{.Config.ClusterAddr.Value}}"
			cluster_name = "{{.Config.ClusterName.Value}}"
			listener = {
				type = "{{.Config.Listener.Type.Value}}"
				attributes = {
					{{range $name, $val := .Config.Listener.Attrs.Value}}
					{{$name}} = "{{$val}}"
					{{end}}
				}
			}
			log_level = "${{.Config.LogLevel.Value}}"
			{{if .Config.Seal.Attrs.Value}}
			seal = {
				type = "{{.Config.Seal.Type.Value}}"
				attributes = {
					{{range $name, $val := .Config.Seal.Attrs.Value}}
					{{$name}} = "{{$val}}"
					{{end}}
				}
			}
			{{end}}
			{{if .Config.Seals}}
			seals = {
			{{range $priority, $seal := .Config.Seals.Value}}
				{{if $seal.Type.Value }}
				{{$priority}} = {
					type = "{{$seal.Type.Value}}"
					attributes = {
						{{range $name, $val := $seal.Attrs.Value}}
						{{$name}} = "{{$val}}"
						{{end}}
					}
				}
				{{end}}
			{{end}}
			}
			{{end}}
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

		{{ renderTransport .Transport }}
	}`))

	cases := []testAccResourceTemplate{}

	vaultStart := newVaultStartStateV1()
	vaultStart.ID.Set("foo")
	vaultStart.BinPath.Set("/opt/vault/bin/vault")
	vaultStart.ConfigDir.Set("/etc/vault.d")
	vaultStart.Config.APIAddr.Set("http://127.0.0.1:8200")
	vaultStart.Config.ClusterAddr.Set("http://127.0.0.1:8201")
	vaultStart.Config.ClusterName.Set("avaultcluster")
	vaultStart.Config.Listener.Set(newVaultConfigBlockSet(
		"tcp", map[string]any{
			"address":     "0.0.0.0:8200",
			"tls_disable": "true",
		}, "config", "listener"))
	vaultStart.Config.LogLevel.Set("debug")
	vaultStart.Config.Storage.Set(newVaultConfigBlockSet("consul", map[string]any{
		"address": "127.0.0.1:8500",
		"path":    "vault",
	}, "config", "storage"))
	vaultStart.Config.Seal.Set(newVaultConfigBlockSet("awskms", map[string]any{
		"kms_key_id": "some-key-id",
	}, "config", "seal"))
	require.NoError(t, vaultStart.Config.Seals.SetSeals(map[string]*vaultConfigBlockSet{
		"primary": newVaultConfigBlockSet("awskms", map[string]any{
			"kms_key_id": "some-key-id",
			"priority":   "1",
		}, "config", "seal"),
		"secondary": newVaultConfigBlockSet("awskms", map[string]any{
			"kms_key_id": "another-key-id",
			"priority":   "2",
		}, "config", "seal"),
	}))
	vaultStart.License.Set("some-license-key")
	vaultStart.SystemdUnitName.Set("vaulter")
	vaultStart.Username.Set("vault")
	ssh := newEmbeddedTransportSSH()
	ssh.User.Set("ubuntu")
	ssh.Host.Set("localhost")
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	ssh.PrivateKey.Set(privateKey)
	require.NoError(t, vaultStart.Transport.SetTransportState(ssh))
	cases = append(cases, testAccResourceTemplate{
		"all fields are loaded correctly",
		vaultStart,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_vault_start.foo", "id", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "bin_path", regexp.MustCompile(`^/opt/vault/bin/vault$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config_dir", regexp.MustCompile(`^/etc/vault.d$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.api_addr", regexp.MustCompile(`^http://127.0.0.1:8200$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.cluster_addr", regexp.MustCompile(`^http://127.0.0.1:8201$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.cluster_name", regexp.MustCompile(`^avaultcluster$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.listener.type", regexp.MustCompile(`^tcp$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.listener.attributes.address", regexp.MustCompile(`^0.0.0.0:8200$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.listener.attributes.tls_disable", regexp.MustCompile(`^true$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.log_level", regexp.MustCompile(`^debug$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.storage.type", regexp.MustCompile(`^consul$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.storage.attributes.address", regexp.MustCompile(`^127.0.0.0:8500$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.storage.attributes.path", regexp.MustCompile(`^vault$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.seal.type", regexp.MustCompile(`^awskms$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.seal.attributes.kms_key_id", regexp.MustCompile(`^some-key-id$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.seals[0].type", regexp.MustCompile(`^awskms$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.seals[0].attributes.kms_key_id", regexp.MustCompile(`^some-key-id$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.seals[0].attributes.priority", regexp.MustCompile(`^1$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.seals[1].type", regexp.MustCompile(`^awskms$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.seals[1].attributes.kms_key_id", regexp.MustCompile(`^another-key-id$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.seals[1].attributes.priority", regexp.MustCompile(`^4$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.license", regexp.MustCompile(`^some-license-key$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.unit_name", regexp.MustCompile(`^vault$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.username", regexp.MustCompile(`^vaulter$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
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
