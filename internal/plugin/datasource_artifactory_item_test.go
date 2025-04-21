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

// TestAccDataSourceArtifacotoryItemProperties is an integration test that uses the
// actual HashiCorp artifactory service to resolve items based on the search
// criteria.
//
//nolint:paralleltest// because our resource handles it
func TestAccDataSourceArtifacotoryItemProperties(t *testing.T) {
	for name, props := range map[string]map[string]string{
		"vault-1.18.5-1.x86_64.rpm": {
			"commit":          "06a36557c4904c52f720bafb71866d389385f5ad",
			"product-name":    "vault",
			"product-version": "1.18.5",
		},
		"vault-1.19.1-1.x86_64.rpm": {
			"build.number": "69bb3e9e943962d8fc2aa8a12031d35d2aac9a68",
			"build.name":   "vault-1.19.1",
		},
	} {
		t.Run(name, func(t *testing.T) {
			state := newArtifactoryItemStateV1()
			_, okacc := os.LookupEnv("TF_ACC")
			username, _ := os.LookupEnv("ARTIFACTORY_USER")
			token, oktoken := os.LookupEnv("ARTIFACTORY_TOKEN")

			if !okacc || !oktoken {
				t.Log(`skipping data "enos_artifactory_item" test because either TF_ACC or ARTIFACTORY_TOKEN are not set`)
				t.Skip()

				return
			}

			state.Username.Set(username)
			state.Token.Set(token)
			state.Host.Set("https://artifactory.hashicorp.engineering/artifactory")
			state.Repo.Set("hashicorp-crt-stable-local*")
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

// TestAccDataSourceArtifacotoryItemQueryTemplate is an integration test that
// uses the actual HashiCorp artifactory service to resolve items based on the
// search criteria.
//
//nolint:paralleltest// because our resource handles it
func TestAccDataSourceArtifacotoryItemQueryTemplate(t *testing.T) {
	orQueryTemplate := template.Must(template.New("or").Parse(`
items.find(
{
  "$or": [
    {
      "$and":
      [
        {"@commit": { "$match": "{{ .SHA }}" }},
        {"@product-name": { "$match": "vault" }},
        {"repo": { "$match": "hashicorp-crt-stable-local*" }},
        {"path": { "$match": "vault/*" }},
        {"name": { "$match": "{{ .Name }}"}}
      ]
    },
    {
      "$and":
      [
        {"@build.number": { "$match": "{{ .SHA }}" }},
        {"@build.name": { "$match": "vault" }},
        {"repo": { "$match": "hashicorp-crt-stable-local*" }},
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
		"vault_1.19.0-1_amd64.deb": "ea8260c5893f2f38c3daa7aed07e89d85613844f",
	} {
		t.Run(name, func(t *testing.T) {
			state := newArtifactoryItemStateV1()
			_, okacc := os.LookupEnv("TF_ACC")
			username, _ := os.LookupEnv("ARTIFACTORY_USER")
			token, oktoken := os.LookupEnv("ARTIFACTORY_TOKEN")

			if !okacc || !oktoken {
				t.Log(`skipping data "enos_artifactory_item" test because either TF_ACC or ARTIFACTORY_TOKEN are not set`)
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
  username = "{{ .Username.Value }}"
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
