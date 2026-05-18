// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"bytes"
	"os"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccDataSourceArtifactoryItemProperties is an integration test that uses the
// actual HashiCorp artifactory service to resolve items based on the search
// criteria.
//
//nolint:paralleltest// because our resource handles it
func TestAccDataSourceArtifactoryItemProperties(t *testing.T) {
	for name, props := range map[string]map[string]string{
		"vault-enterprise-2.0.0+ent-1.x86_64.rpm": {
			"build.number": "f71c0251abe59d87152bb89e726a025f53a45ddc",
			"build.name":   "vault-enterprise-2.0.0+ent",
		},
		"vault-enterprise-2.0.0+ent-1.aarch64.rpm": {
			"commit":          "f71c0251abe59d87152bb89e726a025f53a45ddc",
			"product-name":    "vault-enterprise",
			"product-version": "2.0.0+ent",
		},
	} {
		t.Run(name, func(t *testing.T) {
			state := newArtifactoryItemStateV1()
			_, okacc := os.LookupEnv("TF_ACC")
			var token string
			var okbearertoken, oktoken bool
			username, okuser := os.LookupEnv("ARTIFACTORY_USER")
			if !okuser {
				token, okbearertoken = os.LookupEnv("ARTIFACTORY_BEARER_TOKEN")
			} else {
				token, oktoken = os.LookupEnv("ARTIFACTORY_TOKEN")
			}

			if !okacc || (!oktoken && !okbearertoken) {
				t.Logf(
					`skipping data "enos_artifactory_item" test because one or more of the following isn't set:
					TF_ACC(%t), ARTIFACTORY_TOKEN(%t), ARTIFACTORY_BEARER_TOKEN(%t)`,
					okacc, oktoken, okbearertoken,
				)
				t.Skip()

				return
			}

			state.Username.Set(username)
			state.Token.Set(token)
			state.Host.Set("https://artifactory.hashicorp.engineering/artifactory")
			state.Repo.Set("hashicorp-crt-prod-local*")
			state.Path.Set("vault-enterprise/*")
			state.Name.Set(name)
			state.Properties.SetStrings(props)

			cfg := template.Must(template.New("enos_data_artifactory_item").Parse(`data "enos_artifactory_item" "vault" {
  {{if .Username.Value -}}
  username = "{{ .Username.Value }}"
  {{end -}}
  token    = "{{ .Token.Value }}"

  host = "{{ .Host.Value }}"
  repo = "{{ .Repo.Value }}"
  path = "{{ .Path.Value }}"
  name = "{{ .Name.Value }}"

  {{ if .Properties.StringValue -}}
  properties = {
  {{ range $k, $v := .Properties.StringValue -}}
    "{{ $k }}" = "{{ $v }}"
  {{ end -}}
  }
  {{ end -}}
}

output "name" {
  value = data.enos_artifactory_item.vault.results[0].name
}`))

			buf := bytes.Buffer{}
			require.NoError(t, cfg.Execute(&buf, state))

			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testProviders(t),
				Steps: []resource.TestStep{
					{
						Config: buf.String(),
						Check: resource.ComposeTestCheckFunc(
							resource.TestMatchOutput("name", regexp.MustCompile(`vault\-enterprise`)),
						),
					},
				},
			})
		})
	}
}

// TestAccDataSourceArtifactoryItemQueryTemplate is an integration test that
// uses the actual HashiCorp artifactory service to resolve items based on the
// search criteria.
//
//nolint:paralleltest// because our resource handles it
func TestAccDataSourceArtifactoryItemQueryTemplate(t *testing.T) {
	orQueryTemplate := template.Must(template.New("or").Parse(`
items.find(
{
  "$or": [
    {
      "$and":
      [
        {"@commit": { "$match": "{{ .SHA }}" }},
        {"@product-name": { "$match": "vault-enterprise" }},
        {"repo": { "$match": "hashicorp-crt-prod-local*" }},
        {"path": { "$match": "vault-enterprise/*" }},
        {"name": { "$match": "{{ .Name }}"}}
      ]
    },
    {
      "$and":
      [
        {"@build.number": { "$match": "{{ .SHA }}" }},
        {"@build.name": { "$match": "vault-enterprise-2.0.0+ent" }},
        {"repo": { "$match": "hashicorp-crt-prod-local*" }},
        {"path": { "$match": "vault-enterprise/*" }},
        {"name": { "$match": "{{ .Name }}"}}
      ]
    }
  ]
}).include("*", "property.*") .sort({"$desc": ["modified"]})`))

	for name, sha := range map[string]string{
		"vault-enterprise-2.0.0+ent-1.x86_64.rpm":  "f71c0251abe59d87152bb89e726a025f53a45ddc",
		"vault-enterprise-2.0.0+ent-1.aarch64.rpm": "f71c0251abe59d87152bb89e726a025f53a45ddc",
	} {
		t.Run(name, func(t *testing.T) {
			state := newArtifactoryItemStateV1()
			_, okacc := os.LookupEnv("TF_ACC")
			var token string
			var okbearertoken, oktoken bool
			username, okuser := os.LookupEnv("ARTIFACTORY_USER")
			if !okuser {
				token, okbearertoken = os.LookupEnv("ARTIFACTORY_BEARER_TOKEN")
			} else {
				token, oktoken = os.LookupEnv("ARTIFACTORY_TOKEN")
			}

			if !okacc || (okuser && !oktoken) || (!okuser && !okbearertoken) {
				t.Logf(
					`skipping data "enos_artifactory_item" test because one or more of the following isn't set:
					TF_ACC(%t), ARTIFACTORY_TOKEN(%t), ARTIFACTORY_BEARER_TOKEN(%t)`,
					okacc, oktoken, okbearertoken,
				)
				t.Skip()

				return
			}

			qbuf := &bytes.Buffer{}
			err := orQueryTemplate.Execute(qbuf, struct {
				SHA  string
				Name string
			}{
				SHA:  sha,
				Name: name,
			})
			require.NoError(t, err)

			state.Username.Set(username)
			state.Token.Set(token)
			state.Host.Set("https://artifactory.hashicorp.engineering/artifactory")
			state.QueryTemplate.Set(qbuf.String())

			cfg := template.Must(template.New("enos_data_artifactory_item").Parse(`data "enos_artifactory_item" "vault" {
  {{if .Username.Value -}}
  username = "{{ .Username.Value }}"
  {{end -}}
  token    = "{{ .Token.Value }}"
  host     = "{{ .Host.Value }}"
  query_template = <<EOT
{{ .QueryTemplate.Value }}
  EOT
}

output "name" {
  value = data.enos_artifactory_item.vault.results[0].name
}`))

			buf := bytes.Buffer{}
			require.NoError(t, cfg.Execute(&buf, state))

			resource.ParallelTest(t, resource.TestCase{
				ProtoV6ProviderFactories: testProviders(t),
				Steps: []resource.TestStep{
					{
						Config: buf.String(),
						Check: resource.ComposeTestCheckFunc(
							resource.TestMatchOutput("name", regexp.MustCompile(`vault\-enterprise`)),
						),
					},
				},
			})
		})
	}
}
