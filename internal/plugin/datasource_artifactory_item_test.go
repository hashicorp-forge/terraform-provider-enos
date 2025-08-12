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
		"vault-1.19.1-1.x86_64.rpm": {
			"build.number": "aa75903ec499b2236da9e7bbbfeb7fd16fa4fd9d",
			"build.name":   "vault-1.19.1",
		},
		"vault-1.19.2-1.x86_64.rpm": {
			"commit":          "2ee4ea013b31a770a2fc421bb1e4bc74a9669185",
			"product-name":    "vault",
			"product-version": "1.19.2",
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

			if !okacc || !oktoken {
				t.Logf(`skipping data "enos_artifactory_item" test because one or more of the following isn't set:
					TF_ACC(%t), ARTIFACTORY_TOKEN(%t), ARTIFACTORY_BEARER_TOKEN(%t)`,
					okacc, oktoken, okbearertoken,
				)
				t.Skip()

				return
			}

			state.Username.Set(username)
			state.Token.Set(token)
			state.Host.Set("https://artifactory.hashicorp.engineering/artifactory")
			state.Repo.Set("hashicorp-crt-staging-local*")
			state.Path.Set("vault/*")
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
							resource.TestMatchOutput("name", regexp.MustCompile(name)),
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
        {"@product-name": { "$match": "vault" }},
        {"repo": { "$match": "hashicorp-crt-staging-local*" }},
        {"path": { "$match": "vault/*" }},
        {"name": { "$match": "{{ .Name }}"}}
      ]
    },
    {
      "$and":
      [
        {"@build.number": { "$match": "{{ .SHA }}" }},
        {"@build.name": { "$match": "vault" }},
        {"repo": { "$match": "hashicorp-crt-staging-local*" }},
        {"path": { "$match": "vault/*" }},
        {"name": { "$match": "{{ .Name }}"}}
      ]
    }
  ]
}).include("*", "property.*") .sort({"$desc": ["modified"]})`))

	for name, sha := range map[string]string{
		// New CRT fields
		"vault_1.19.1-1_amd64.deb": "aa75903ec499b2236da9e7bbbfeb7fd16fa4fd9d",
		// Old CRT fields
		"vault_1.19.0-1_amd64.deb": "7eeafb6160d60ede73c1d95566b0c8ea54f3cb5a",
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
				t.Logf(`skipping data "enos_artifactory_item" test because one or more of the following isn't set:
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
							resource.TestMatchOutput("name", regexp.MustCompile(name)),
						),
					},
				},
			})
		})
	}
}
