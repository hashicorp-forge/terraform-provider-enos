package plugin

import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

var testProviders = map[string]func() (tfprotov5.ProviderServer, error){
	"enos": func() (tfprotov5.ProviderServer, error) {
		return Server(), nil
	},
}
