// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package artifactory

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
)

func EnsureArtifactoryEnvAuth(t *testing.T) (map[string]string, bool) {
	t.Helper()

	var okacc, oktoken, okbearertoken, okuser bool
	vars := map[string]string{}

	_, okacc = os.LookupEnv("TF_ACC")
	vars["username"], okuser = os.LookupEnv("ARTIFACTORY_USER")
	if !okuser {
		vars["token"], okbearertoken = os.LookupEnv("ARTIFACTORY_BEARER_TOKEN")
	} else {
		vars["token"], oktoken = os.LookupEnv("ARTIFACTORY_TOKEN")
	}

	if !okacc || (okuser && !oktoken) || (!okuser && !okbearertoken) {
		t.Logf(`skipping data "enos_artifactory_item" test because one or more of the following isn't set:
			TF_ACC(%t), ARTIFACTORY_TOKEN(%t), ARTIFACTORY_BEARER_TOKEN(%t)`,
			okacc, oktoken, okbearertoken,
		)
		t.Skip()

		return vars, false
	}

	return vars, true
}

func EnsureArtifactoryEnvVars(t *testing.T) (map[string]string, bool) {
	t.Helper()

	var okacc, oktoken, okbearertoken, okver, okrev, okuser bool
	vars := map[string]string{}

	_, okacc = os.LookupEnv("TF_ACC")
	vars["username"], okuser = os.LookupEnv("ARTIFACTORY_USER")
	if !okuser {
		vars["token"], okbearertoken = os.LookupEnv("ARTIFACTORY_BEARER_TOKEN")
	} else {
		vars["token"], oktoken = os.LookupEnv("ARTIFACTORY_TOKEN")
	}
	vars["version"], okver = os.LookupEnv("ARTIFACTORY_PRODUCT_VERSION")
	vars["revision"], okrev = os.LookupEnv("ARTIFACTORY_REVISION")

	if !okacc || (okuser && !oktoken) || (!okuser && !okbearertoken) || !okver || !okrev {
		t.Logf(`skipping data "enos_artifactory_item" test because one or more of the following isn't set:
			TF_ACC(%t), ARTIFACTORY_TOKEN(%t), ARTIFACTORY_BEARER_TOKEN(%t), ARTIFACTORY_PRODUCT_VERSION(%t), ARTIFACTORY_REVISION(%t)`,
			okacc, oktoken, okbearertoken, okver, okrev,
		)
		t.Skip()

		return vars, false
	}

	return vars, true
}

func TestAccSearchAQL(t *testing.T) {
	t.Parallel()

	vars, _ := EnsureArtifactoryEnvVars(t)

	opts := []Opt{
		WithHost("https://artifactory.hashicorp.engineering/artifactory"),
		WithToken(vars["token"]),
	}
	if vars["username"] != "" {
		opts = append(opts, WithUsername(vars["username"]))
	}
	client := NewClient(opts...)

	for _, test := range []struct {
		Name string
		Args []SearchAQLOpt
	}{
		{
			Name: "all search fields",
			Args: []SearchAQLOpt{
				WithRepo("hashicorp-crt-staging-local*"),
				WithPath("vault/*"),
				WithName("*.zip"),
				WithProperties(map[string]string{
					"product-name":    "vault",
					"product-version": vars["version"],
					"commit":          vars["revision"],
				}),
			},
		},
		{
			Name: "no repo",
			Args: []SearchAQLOpt{
				WithPath("vault/*"),
				WithName("*.zip"),
				WithProperties(map[string]string{
					"product-name":    "vault",
					"product-version": vars["version"],
					"commit":          vars["revision"],
				}),
			},
		},
		{
			Name: "no path",
			Args: []SearchAQLOpt{
				WithRepo("hashicorp-crt-staging-local*"),
				WithName("*.zip"),
				WithProperties(map[string]string{
					"product-name":    "vault",
					"product-version": vars["version"],
					"commit":          vars["revision"],
				}),
			},
		},
		{
			Name: "no name",
			Args: []SearchAQLOpt{
				WithRepo("hashicorp-crt-staging-local*"),
				WithPath("vault/*"),
				WithProperties(map[string]string{
					"product-name":    "vault",
					"product-version": vars["version"],
					"commit":          vars["revision"],
				}),
			},
		},
		{
			Name: "no properties",
			Args: []SearchAQLOpt{
				WithRepo("hashicorp-crt-staging-local*"),
				WithPath("vault/*"),
				WithName("*.zip"),
			},
		},
		{
			Name: "only properties",
			Args: []SearchAQLOpt{
				WithProperties(map[string]string{
					"product-name":    "vault",
					"product-version": vars["version"],
					"commit":          vars["revision"],
				}),
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			test.Args = append(test.Args, WithLimit("1"))
			req := NewSearchAQLRequest(test.Args...)
			res, err := client.SearchAQL(t.Context(), req)
			require.NoError(t, err)
			require.NotEmpty(t, res.Results)
			if req.Name != "" {
				require.Contains(t, req.Name, filepath.Ext(res.Results[0].Name))
			}
			require.NotEmpty(t, res.Results[0].Name)
			require.NotEmpty(t, res.Results[0].Path)
			require.NotEmpty(t, res.Results[0].Repo)
			require.NotEmpty(t, res.Results[0].SHA256)
			require.NotEmpty(t, res.Results[0].Size)
			require.NotEmpty(t, res.Results[0].Type)
			require.NotEmpty(t, res.Results[0].Properties)
		})
	}
}

// TestAccSearchRawQuery tests a raw query. Here we run it with an "$or" query
// that is not supported by the standard property based search. We use it to
// search for artifacts that old and new CRT properties.
func TestAccSearchRawQuery(t *testing.T) {
	t.Parallel()

	vars, _ := EnsureArtifactoryEnvAuth(t)

	opts := []Opt{
		WithHost("https://artifactory.hashicorp.engineering/artifactory"),
		WithToken(vars["token"]),
	}
	if vars["username"] != "" {
		opts = append(opts, WithUsername(vars["username"]))
	}
	client := NewClient(opts...)

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
}).include("*", "property.*") .sort({"$desc": ["modified"]})
`))

	for _, test := range []struct {
		Name         string
		ArtifactName string
		SHA          string
	}{
		// These artifacts have different properties. Search for both and our query
		// should work for both.
		{
			Name:         "new crt fields",
			ArtifactName: "vault_1.19.1-1_amd64.deb",
			SHA:          "aa75903ec499b2236da9e7bbbfeb7fd16fa4fd9d",
		},
		{
			Name:         "old crt fields",
			ArtifactName: "vault_1.19.0-1_amd64.deb",
			SHA:          "7eeafb6160d60ede73c1d95566b0c8ea54f3cb5a",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			err := orQueryTemplate.Execute(buf, struct {
				SHA  string
				Name string
			}{
				SHA:  test.SHA,
				Name: test.ArtifactName,
			})
			queryTemplate := template.Must(template.New("query").Parse(buf.String()))
			require.NoError(t, err)
			req := NewSearchAQLRequest(
				WithLimit("1"),
				WithQueryTemplate(queryTemplate),
			)
			res, err := client.SearchAQL(t.Context(), req)
			require.NoError(t, err)
			require.NotEmpty(t, res.Results)
			require.Equal(t, test.ArtifactName, res.Results[0].Name)
			require.NotEmpty(t, res.Results[0].Path)
			require.NotEmpty(t, res.Results[0].Repo)
			require.NotEmpty(t, res.Results[0].SHA256)
			require.NotEmpty(t, res.Results[0].Size)
			require.NotEmpty(t, res.Results[0].Type)
			require.NotEmpty(t, res.Results[0].Properties)
		})
	}
}
