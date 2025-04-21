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
	vaultCfg.Storage.Set(newVaultStorageConfigSet("raft", map[string]any{
		"address": "127.0.0.1:8500",
		"path":    "vault",
	}, map[string]any{"autojoin": &tfString{Val: "provider=aws tag=thing value=foo"}}))
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
	vaultStart.Config.Storage.Set(newVaultStorageConfigSet("raft", map[string]any{
		"path": "vault",
	}, map[string]any{
		"auto_join":        "provider=aws tag_key=join tag_value=vault",
		"auto_join_scheme": "https",
	}))
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
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.storage.type", regexp.MustCompile(`^raft$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.storage.attributes.path", regexp.MustCompile(`^vault$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.storage.retry_join.auto_join_scheme", regexp.MustCompile(`^https$`)),
			resource.TestMatchResourceAttr("enos_vault_start.foo", "config.storage.retry_join.auto_join", regexp.MustCompile(`^provider=aws tag_key=join tag_value=vault$`)),
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

// Test_sealAttrsToEnvVars tests converting our seal attributes into their respective values.
func Test_sealAttrsToEnvVars(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		in         map[string]any
		expected   map[string]string
		shouldFail bool
	}{
		"alicloudkms": {
			in: map[string]any{
				"region":     "us-east-1",
				"access_key": "AKIAIOSFODNN7EXAMPLE",
				"secret_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"kms_key_id": "19ec80b0-dfdd-4d97-8164-c6examplekey",
			},
			expected: map[string]string{
				"ALICLOUD_REGION":               "us-east-1",
				"ALICLOUD_ACCESS_KEY":           "AKIAIOSFODNN7EXAMPLE",
				"ALICLOUD_SECRET_KEY":           "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"VAULT_ALICLOUDKMS_SEAL_KEY_ID": "19ec80b0-dfdd-4d97-8164-c6examplekey",
				"VAULT_SEAL_TYPE":               "alicloudkms",
			},
		},
		"awskms": {
			in: map[string]any{
				"region":     "us-east-1",
				"access_key": "0wNEpMMlzy7szvai",
				"secret_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"kms_key_id": "19ec80b0-dfdd-4d97-8164-c6examplekey",
				"endpoint":   "https://vpce-0e1bb1852241f8cc6-pzi0do8n.kms.us-east-1.vpce.amazonaws.com",
				"priority":   "1",
				"name":       "primary",
			},
			expected: map[string]string{
				"AWS_REGION":               "us-east-1",
				"AWS_ACCESS_KEY":           "0wNEpMMlzy7szvai",
				"AWS_SECRET_ACCESS_KEY":    "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"AWS_ENDPOINT":             "https://vpce-0e1bb1852241f8cc6-pzi0do8n.kms.us-east-1.vpce.amazonaws.com",
				"VAULT_AWSKMS_SEAL_KEY_ID": "19ec80b0-dfdd-4d97-8164-c6examplekey",
				"VAULT_SEAL_TYPE":          "awskms",
			},
		},
		"azurekeyvault": {
			in: map[string]any{
				"tenant_id":     "46646709-b63e-4747-be42-516edeaf1e14",
				"client_id":     "03dc33fc-16d9-4b77-8152-3ec568f8af6e",
				"client_secret": "DUJDS3...",
				"environment":   "AZUREPUBLICCLOUD",
				"resource":      "vault.azure.net",
				"vault_name":    "hc-vault",
				"key_name":      "vault_key",
				"priority":      "1",
				"name":          "primary",
			},
			expected: map[string]string{
				"AZURE_TENANT_ID":                "46646709-b63e-4747-be42-516edeaf1e14",
				"AZURE_CLIENT_ID":                "03dc33fc-16d9-4b77-8152-3ec568f8af6e",
				"AZURE_CLIENT_SECRET":            "DUJDS3...",
				"AZURE_ENVIRONMENT":              "AZUREPUBLICCLOUD",
				"AZURE_AD_RESOURCE":              "vault.azure.net",
				"VAULT_AZUREKEYVAULT_VAULT_NAME": "hc-vault",
				"VAULT_AZUREKEYVAULT_KEY_NAME":   "vault_key",
				"VAULT_SEAL_TYPE":                "azurekeyvault",
			},
		},
		"gcpckms": {
			in: map[string]any{
				"credentials": "/usr/vault/vault-project-user-creds.json",
				"project":     "vault-project",
				"region":      "global",
				"key_ring":    "vault-keyring",
				"crypto_key":  "vault-key",
				"priority":    "1",
				"name":        "primary",
			},
			expected: map[string]string{
				"GOOGLE_CREDENTIALS":            "/usr/vault/vault-project-user-creds.json",
				"GOOGLE_PROJECT":                "vault-project",
				"GOOGLE_REGION":                 "global",
				"VAULT_GCPCKMS_SEAL_KEY_RING":   "vault-keyring",
				"VAULT_GCPCKMS_SEAL_CRYPTO_KEY": "vault-key",
				"VAULT_SEAL_TYPE":               "gcpckms",
			},
		},
		"pkcs11": {
			in: map[string]any{
				"default_key_label":      "ignored",
				"default_hmac_key_label": "ignored_hmac",
				"force_rw_session":       "true",
				"generate_key":           "true",
				"key_id":                 "0xba5eba11",
				"hmac_key_id":            "0x33333435363434373537",
				"hmac_key_label":         "vault-hsm-hmac-key",
				"hmac_mechanism":         "0x0251",
				"key_label":              "vault-hsm-key",
				"lib":                    "/usr/vault/lib/libCryptoki2_64.so",
				"max_parallel":           1,
				"mechanism":              "0x1085",
				"pin":                    "AAAA-BBBB-CCCC-DDDD",
				"rsa_encrypt_local":      "true",
				"rsa_oaep_hash":          "sha256",
				"slot":                   "2305843009213693953",
				"token_label":            "vault-token-label",
				"priority":               "1",
				"name":                   "primary",
			},
			expected: map[string]string{
				"VAULT_HSM_DEFAULT_KEY_LABEL":      "ignored",
				"VAULT_HSM_FORCE_RW_SESSION":       "true",
				"VAULT_HSM_GENERATE_KEY":           "true",
				"VAULT_HSM_HMAC_DEFAULT_KEY_LABEL": "ignored_hmac",
				"VAULT_HSM_HMAC_KEY_LABEL":         "vault-hsm-hmac-key",
				"VAULT_HSM_HMAC_KEY_ID":            "0x33333435363434373537",
				"VAULT_HSM_HMAC_MECHANISM":         "0x0251",
				"VAULT_HSM_KEY_ID":                 "0xba5eba11",
				"VAULT_HSM_KEY_LABEL":              "vault-hsm-key",
				"VAULT_HSM_LIB":                    "/usr/vault/lib/libCryptoki2_64.so",
				// VAULT_HSM_MAX_PARALLEL might not do anything from the env
				"VAULT_HSM_MAX_PARALLEL":      "1",
				"VAULT_HSM_MECHANISM":         "0x1085",
				"VAULT_HSM_PIN":               "AAAA-BBBB-CCCC-DDDD",
				"VAULT_HSM_RSA_ENCRYPT_LOCAL": "true",
				"VAULT_HSM_RSA_OAEP_HASH":     "sha256",
				"VAULT_HSM_SLOT":              "2305843009213693953",
				"VAULT_HSM_TOKEN_LABEL":       "vault-token-label",
				"VAULT_SEAL_TYPE":             "pkcs11",
			},
		},
		"ocikms": {
			in: map[string]any{
				"key_id":              "ocid1.key.oc1.iad.afnxza26aag4s.abzwkljsbapzb2nrha5nt3s7s7p42ctcrcj72vn3kq5qx",
				"crypto_endpoint":     "https://afnxza26aag4s-crypto.kms.us-ashburn-1.oraclecloud.com",
				"management_endpoint": "https://afnxza26aag4s-management.kms.us-ashburn-1.oraclecloud.com",
				"auth_type_api_key":   "true",
				"priority":            "1",
				"name":                "primary",
			},
			expected: map[string]string{
				"VAULT_OCIKMS_SEAL_KEY_ID":         "ocid1.key.oc1.iad.afnxza26aag4s.abzwkljsbapzb2nrha5nt3s7s7p42ctcrcj72vn3kq5qx",
				"VAULT_OCIKMS_CRYPTO_ENDPOINT":     "https://afnxza26aag4s-crypto.kms.us-ashburn-1.oraclecloud.com",
				"VAULT_OCIKMS_MANAGEMENT_ENDPOINT": "https://afnxza26aag4s-management.kms.us-ashburn-1.oraclecloud.com",
				// VAULT_OCIKMS_AUTH_TYPE_API_KEY may not actually do anything
				"VAULT_OCIKMS_AUTH_TYPE_API_KEY": "true",
				"VAULT_SEAL_TYPE":                "ocikms",
			},
		},
		"transit": {
			in: map[string]any{
				"address":         "https://vault:8200",
				"token":           "s.Qf1s5zigZ4OX6akYjQXJC1jY",
				"disable_renewal": "false",
				"key_name":        "transit_key_name",
				"key_id_prefix":   "transit_key_id_prefix",
				"mount_path":      "transit/",
				"namespace":       "ns1/",
				"tls_ca_cert":     "/etc/vault/ca_cert.pem",
				"tls_client_cert": "/etc/vault/client_cert.pem",
				"tls_client_key":  "/etc/vault/ca_cert.pem",
				"tls_server_name": "vault",
				"tls_skip_verify": "false",
				"priority":        "1",
				"name":            "primary",
			},
			expected: map[string]string{
				"VAULT_ADDR":                         "https://vault:8200",
				"VAULT_TOKEN":                        "s.Qf1s5zigZ4OX6akYjQXJC1jY",
				"VAULT_TRANSIT_SEAL_DISABLE_RENEWAL": "false",
				"VAULT_TRANSIT_SEAL_KEY_NAME":        "transit_key_name",
				"VAULT_TRANSIT_SEAL_MOUNT_PATH":      "transit/",
				"VAULT_NAMESPACE":                    "ns1/",
				"VAULT_CA_CERT":                      "/etc/vault/ca_cert.pem",
				"VAULT_CLIENT_CERT":                  "/etc/vault/client_cert.pem",
				"VAULT_CLIENT_KEY":                   "/etc/vault/ca_cert.pem",
				"VAULT_TLS_SERVER_NAME":              "vault",
				"VAULT_SKIP_VERIFY":                  "false",
				"VAULT_SEAL_TYPE":                    "transit",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			trans := sealAttrEnvVarTranslator{}
			got, err := trans.ToEnvVars(name, test.in)
			if test.shouldFail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.expected, got)
		})
	}
}
