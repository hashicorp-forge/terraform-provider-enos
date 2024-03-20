// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package artifactory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func EnsureArtifactoryEnvVars(t *testing.T) (map[string]string, bool) {
	t.Helper()

	var okacc, okuser, oktoken, okver, okrev bool
	vars := map[string]string{}

	_, okacc = os.LookupEnv("TF_ACC")
	vars["username"], okuser = os.LookupEnv("ARTIFACTORY_USER")
	vars["token"], oktoken = os.LookupEnv("ARTIFACTORY_TOKEN")
	vars["version"], okver = os.LookupEnv("ARTIFACTORY_PRODUCT_VERSION")
	vars["revision"], okrev = os.LookupEnv("ARTIFACTORY_REVISION")

	if !(okacc && okuser && oktoken && okver && okrev) {
		t.Log(`skipping data "enos_artifactory_item" test because TF_ACC, ARTIFACTORY_TOKEN, ARTIFACTORY_USER, ARTIFACTORY_PRODUCT_VERSION, ARTIFACTORY_REVISION aren't set`)
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
				WithRepo("hashicorp-packagespec-buildcache-local*"),
				WithPath("cache-v1/vault-enterprise/*"),
				WithName("*.zip"),
				WithProperties(map[string]string{
					"artifactType":    "package",
					"productVersion":  vars["version"],
					"productRevision": vars["revision"],
					"GOOS":            "linux",
					"GOARCH":          "amd64",
					"EDITION":         "ent",
				}),
			},
		},
		{
			Name: "no repo",
			Args: []SearchAQLOpt{
				WithPath("cache-v1/vault-enterprise/*"),
				WithName("*.zip"),
				WithProperties(map[string]string{
					"artifactType":    "package",
					"productVersion":  vars["version"],
					"productRevision": vars["revision"],
					"GOOS":            "linux",
					"GOARCH":          "amd64",
					"EDITION":         "ent",
				}),
			},
		},
		{
			Name: "no path",
			Args: []SearchAQLOpt{
				WithRepo("hashicorp-packagespec-buildcache-local*"),
				WithName("*.zip"),
				WithProperties(map[string]string{
					"artifactType":    "package",
					"productVersion":  vars["version"],
					"productRevision": vars["revision"],
					"GOOS":            "linux",
					"GOARCH":          "amd64",
					"EDITION":         "ent",
				}),
			},
		},
		{
			Name: "no name",
			Args: []SearchAQLOpt{
				WithRepo("hashicorp-packagespec-buildcache-local*"),
				WithPath("cache-v1/vault-enterprise/*"),
				WithProperties(map[string]string{
					"artifactType":    "package",
					"productVersion":  vars["version"],
					"productRevision": vars["revision"],
					"GOOS":            "linux",
					"GOARCH":          "amd64",
					"EDITION":         "ent",
				}),
			},
		},
		{
			Name: "no properties",
			Args: []SearchAQLOpt{
				WithRepo("hashicorp-packagespec-buildcache-local*"),
				WithPath("cache-v1/vault-enterprise/*"),
				WithName("*.zip"),
			},
		},
		{
			Name: "only properties",
			Args: []SearchAQLOpt{
				WithProperties(map[string]string{
					"artifactType":    "package",
					"productVersion":  vars["version"],
					"productRevision": vars["revision"],
					"GOOS":            "linux",
					"GOARCH":          "amd64",
					"EDITION":         "ent",
				}),
			},
		},
	} {
		test := test
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			test.Args = append(test.Args, WithLimit("1"))
			req := NewSearchAQLRequest(test.Args...)
			res, err := client.SearchAQL(context.Background(), req)
			require.NoError(t, err)
			require.NotEmpty(t, res.Results)
			require.Equal(t, ".zip", filepath.Ext(res.Results[0].Name))
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
