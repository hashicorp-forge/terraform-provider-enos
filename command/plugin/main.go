package main

import (
	"github.com/hashicorp/enos-provider/internal/plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
)

func main() {
	err := tf6server.Serve("hashicorp.com/qti/enos", func() tfprotov6.ProviderServer {
		return plugin.Server()
	})
	if err != nil {
		panic(err)
	}
}
