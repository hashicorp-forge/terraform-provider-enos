package plugin

import (
	"bytes"
	"regexp"
	"testing"
	"text/template"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccResourceKindLoadImageValidAttributes(t *testing.T) {
	cfg := template.Must(template.New("kind_cluster").Parse(`resource "enos_local_kind_load_image" "{{.ID.Value}}" {
		{{if .ClusterName.Value}}
		cluster_name = "{{.ClusterName.Value}}"
		{{end}}
		{{if .Image.Value}}
		image = "{{.Image.Value}}"
		{{end}}
		{{if .Tag.Value}}
		tag = "{{.Tag.Value}}"
		{{end}}
	}`))

	loadImageState := newLocalKindLoadImageStateV1()
	loadImageState.Image.Set("bananas")
	loadImageState.Tag.Set("0.1.0")
	loadImageState.ID.Set("bananas")
	loadImageState.ClusterName.Set("funky-chicken")

	test := testAccResourceTemplate{
		name:  "valid_attributes",
		state: loadImageState,
		check: resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_kind_load_image.bananas", "id", regexp.MustCompile(`^bananas$`)),
			resource.TestMatchResourceAttr("enos_kind_load_image.bananas", "image", regexp.MustCompile(`^bananas$`)),
			resource.TestMatchResourceAttr("enos_kind_load_image.bananas", "tag", regexp.MustCompile(`^0.1.0$`)),
			resource.TestMatchResourceAttr("enos_kind_load_image.bananas", "cluster_name", regexp.MustCompile(`^funky-chicken$`)),
		),
		apply: false,
	}

	buf := bytes.Buffer{}
	err := cfg.Execute(&buf, test.state)
	if err != nil {
		t.Fatalf("error executing test template: %s", err.Error())
	}

	step := resource.TestStep{
		Config:             buf.String(),
		Check:              test.check,
		PlanOnly:           true,
		ExpectNonEmptyPlan: true,
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviders(t),
		Steps:                    []resource.TestStep{step},
	})
}
