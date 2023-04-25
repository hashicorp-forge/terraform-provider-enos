package plugin

import (
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourceEnvironment(t *testing.T) {
	_, okacc := os.LookupEnv("TF_ACC")

	if !okacc {
		t.Log(`skipping data "enos_environment" test because TF_ACC isn't set`)
		t.Skip()
		return
	}

	cfg := `data "enos_environment" "localhost" { }`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviders(t),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("data.enos_environment.localhost", "public_ip_address", regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)),
					resource.TestMatchResourceAttr("data.enos_environment.localhost", "public_ip_addresses.0", regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)),
					resource.TestMatchResourceAttr("data.enos_environment.localhost", "public_ipv4_addresses.0", regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)),
				),
			},
		},
	})
}
