package plugin

import (
	"github.com/hashicorp/enos-provider/internal/server"
	"github.com/hashicorp/enos-provider/internal/server/datarouter"
	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

// Server returns a default instance of our ProviderServer
func Server() tfprotov5.ProviderServer {
	return server.New(
		server.RegisterProvider(NewProvider()),
		server.RegisterDataRouter(datarouter.New(
			datarouter.RegisterDataSource(newTransport()),
		)),
		server.RegisterResourceRouter(resourcerouter.New(
			resourcerouter.RegisterResource(newFile()),
			resourcerouter.RegisterResource(newRemoteExec()),
		)),
	)
}
