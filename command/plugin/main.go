package main

import (
	"os"

	"github.com/hashicorp/enos-provider/internal/plugin"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
)

func main() {
	// setup debug mode if the provider is launched with the flag --debug
	var opts []tf6server.ServeOpt
	for _, arg := range os.Args[1:] {
		if arg == "--debug" {
			opts = append(opts, tf6server.WithManagedDebug())
		}
	}

	err := tf6server.Serve("hashicorp.com/qti/enos", func() tfprotov6.ProviderServer {
		return plugin.Server()
	}, opts...)
	if err != nil {
		panic(err)
	}
}
