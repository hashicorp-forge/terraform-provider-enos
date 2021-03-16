package plugin

import (
	"github.com/hashicorp/enos-provider/internal/server"
	dr "github.com/hashicorp/enos-provider/internal/server/datarouter"
	rr "github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

// Server returns a default instance of our ProviderServer
func Server() tfprotov5.ProviderServer {
	return server.New(
		server.RegisterProvider(newProvider()),
		server.RegisterDataRouter(dr.New(
			dr.RegisterDataSource(newTransport()),
		)),
		server.RegisterResourceRouter(rr.New(
			rr.RegisterResource(newFile()),
			rr.RegisterResource(newRemoteExec()),
		)),
	)
}
