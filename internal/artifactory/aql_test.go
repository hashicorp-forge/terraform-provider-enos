// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package artifactory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func EnsureArtifactoryEnvVars(t *testing.T) (map[string]string, bool) {
	t.Helper()

	var okacc, oktoken, okver, okrev bool
	vars := map[string]string{}

	_, okacc = os.LookupEnv("TF_ACC")
	vars["username"], _ = os.LookupEnv("ARTIFACTORY_USER")
	vars["token"], oktoken = os.LookupEnv("ARTIFACTORY_TOKEN")
	vars["version"], okver = os.LookupEnv("ARTIFACTORY_PRODUCT_VERSION")
	vars["revision"], okrev = os.LookupEnv("ARTIFACTORY_REVISION")

	if !(okacc && oktoken && okver && okrev) {
		t.Logf(`skipping data "enos_artifactory_item" test because TF_ACC(%t), ARTIFACTORY_TOKEN(%t), ARTIFACTORY_PRODUCT_VERSION(%t), ARTIFACTORY_REVISION(%t) aren't set`,
			okacc, oktoken, okver, okrev,
		)
		t.Skip()

		return vars, false
	}

	return vars, true
}

func TestAccSearchAQL(t *testing.T) {
	t.Parallel()

	vars, _ := EnsureArtifactoryEnvVars(t)

	client := NewClient(
		WithHost("https://artifactory.hashicorp.engineering/artifactory"),
		WithUsername(vars["username"]),
		WithToken(vars["token"]),
	)

	for _, test := range []struct {
		Name string
		Args []SearchAQLOpt
	}{
		{
			Name: "all search fields",
			Args: []SearchAQLOpt{
				WithRepo("hashicorp-crt-stable-local*"),
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
				WithRepo("hashicorp-crt-stable-local*"),
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
				WithRepo("hashicorp-crt-stable-local*"),
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
				WithRepo("hashicorp-crt-stable-local*"),
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
