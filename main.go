package main

import (
	"github.com/hashicorp/enos-provider/internal/plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	tf5server "github.com/hashicorp/terraform-plugin-go/tfprotov5/server"
)

func main() {
	err := tf5server.Serve("hashicorp.com/qti/enos", func() tfprotov5.ProviderServer {
		return plugin.Server()
	})
	if err != nil {
		panic(err)
	}
}
