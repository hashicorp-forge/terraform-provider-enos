package plugin

import (
	"context"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestPublicIPAddressResolver(t *testing.T) {
	_, ok := os.LookupEnv("TF_ACC")
	if !ok {
		t.Log("skipping public ip address resolution because TF_ACC is not set")
		t.Skip()
		return
	}

	pubip := newPublicIPResolver()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5*time.Second))
	defer cancel()

	openIP, err := pubip.resolveOpenDNS(ctx)
	require.NoError(t, err)

	googleIP, err := pubip.resolveGoogle(ctx)
	require.NoError(t, err)

	awsIP, err := pubip.resolveAWS(ctx)
	require.NoError(t, err)

	require.Equal(t, openIP, googleIP)
	require.Equal(t, openIP, awsIP)
}

func TestAccDataSourceEnvironment(t *testing.T) {
	cfg := `data "enos_environment" "localhost" { }`

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviders(t),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("data.enos_environment.localhost", "public_ip_address", regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)),
				),
			},
		},
	})
}
