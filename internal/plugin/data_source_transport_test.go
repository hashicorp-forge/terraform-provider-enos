package plugin

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourceTransport(t *testing.T) {
	testAccDataSourceTransporft := `data "enos_transport" "foo" {
	ssh {
		user = "ubuntu"
		host = "hostname"
		private_key = "BEGIN PRIVATE KEY"
	}
}`

	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceTransporft,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("data.enos_transport.foo", "ssh.user", regexp.MustCompile(`^ubuntu$`)),
					resource.TestMatchResourceAttr("data.enos_transport.foo", "ssh.host", regexp.MustCompile(`^hostname$`)),
					resource.TestMatchResourceAttr("data.enos_transport.foo", "ssh.private_key", regexp.MustCompile(`^BEGIN PRIVATE KEY$`)),
				),
			},
		},
	})
}
