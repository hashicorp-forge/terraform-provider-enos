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
func TestAccDataSourceArtifacotoryItem(t *testing.T) {
	t.Parallel()

	state := newArtifactoryItemStateV1()
	_, okacc := os.LookupEnv("TF_ACC")
	username, okuser := os.LookupEnv("ARTIFACTORY_USER")
	token, oktoken := os.LookupEnv("ARTIFACTORY_TOKEN")
	version, okver := os.LookupEnv("ARTIFACTORY_PRODUCT_VERSION")
	revision, okrev := os.LookupEnv("ARTIFACTORY_REVISION")

	if !(okacc && okuser && oktoken && okver && okrev) {
		t.Log(`skipping data "enos_artifactory_item" test because TF_ACC, ARTIFACTORY_TOKEN, ARTIFACTORY_USER, ARTIFACATORY_PRODUCT_VERSION, ARTIFACTORY_REVISION aren't set`)
		t.Skip()
		return
	}

	state.Username = username
	state.Token = token
	state.Host = "https://artifactory.hashicorp.engineering/artifactory"
	state.Repo = "hashicorp-packagespec-buildcache-local*"
	state.Path = "cache-v1/vault-enterprise/*"
	state.Name = "*.zip"
	state.Properties["artifactType"] = "package"
	state.Properties["productVersion"] = version
	state.Properties["productRevision"] = revision
	state.Properties["GOOS"] = "linux"
	state.Properties["GOARCH"] = "amd64"
	state.Properties["EDITION"] = "ent"

	cfg := template.Must(template.New("enos_data_artifactory_item").Parse(`data "enos_artifactory_item" "vault" {
  username = "{{ .Username }}"
  token    = "{{ .Token }}"

  host = "{{ .Host }}"
  repo = "{{ .Repo }}"
  path = "{{ .Path }}"
  name = "{{ .Name }}"

  {{ if .Properties -}}
  properties = {
	{{ range $k, $v := .Properties -}}
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

	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testProviders,
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
