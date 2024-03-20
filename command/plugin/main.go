// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	flag "github.com/spf13/pflag"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/plugin"
)

const defaultProviderName = "app.terraform.io/hashicorp-qti/enos"

func main() {
	// setup debug mode if the provider is launched with the flag --debug
	var opts []tf6server.ServeOpt

	debug := flag.BoolP("debug", "d", false, "--debug (-d) - enables debug mode")
	name := flag.StringP("name", "n", defaultProviderName, "--name (-n) <provider name>")

	flag.Parse()

	if *debug {
		opts = append(opts, tf6server.WithManagedDebug())
	}

	err := tf6server.Serve(*name, func() tfprotov6.ProviderServer {
		return plugin.Server()
	}, opts...)
	if err != nil {
		panic(err)
	}
}
