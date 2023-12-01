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

// TestAccDataSourceArtifacotoryItem is an integration test that uses the
// actual HashiCorp artifactory service to resolve items based on the search
// criteria.
//
//nolint:paralleltest// because our resource handles it
func TestAccDataSourceArtifacotoryItem(t *testing.T) {
	state := newArtifactoryItemStateV1()
	_, okacc := os.LookupEnv("TF_ACC")
	username, okuser := os.LookupEnv("ARTIFACTORY_USER")
	token, oktoken := os.LookupEnv("ARTIFACTORY_TOKEN")
	version, okver := os.LookupEnv("ARTIFACTORY_PRODUCT_VERSION")
	revision, okrev := os.LookupEnv("ARTIFACTORY_REVISION")

	if !okacc || !okuser || !oktoken || !okver || !okrev {
		t.Log(`skipping data "enos_artifactory_item" test because TF_ACC, ARTIFACTORY_TOKEN, ARTIFACTORY_USER, ARTIFACATORY_PRODUCT_VERSION, ARTIFACTORY_REVISION aren't set`)
		t.Skip()

		return
	}

	state.Username.Set(username)
	state.Token.Set(token)
	state.Host.Set("https://artifactory.hashicorp.engineering/artifactory")
	state.Repo.Set("hashicorp-packagespec-buildcache-local*")
	state.Path.Set("cache-v1/vault-enterprise/*")
	state.Name.Set("*.zip")
	state.Properties.SetStrings(map[string]string{
		"artifactType":    "package",
		"productVersion":  version,
		"productRevision": revision,
		"GOOS":            "linux",
		"GOARCH":          "amd64",
		"EDITION":         "ent",
	})

	cfg := template.Must(template.New("enos_data_artifactory_item").Parse(`data "enos_artifactory_item" "vault" {
  username = "{{ .Username.Value }}"
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

output "url" {
  value = data.enos_artifactory_item.vault.results[0].name
}
`))

	buf := bytes.Buffer{}
	require.NoError(t, cfg.Execute(&buf, state))

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviders(t),
		Steps: []resource.TestStep{
			{
				Config: buf.String(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchOutput("url", regexp.MustCompile(`.zip`)),
				),
			},
		},
	})
}
