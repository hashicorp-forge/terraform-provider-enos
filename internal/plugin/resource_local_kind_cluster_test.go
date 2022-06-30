package plugin

import (
	"bytes"
	"context"
	"crypto/rand"
	"math/big"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceKindCluster tests the kind_cluster resource
func TestAccResourceKindClusterValidAttributes(t *testing.T) {
	cfg := template.Must(template.New("kind_cluster").Parse(`resource "enos_local_kind_cluster" "{{.ID.Value}}" {
		{{if .Name.Value}}
		name = "{{.Name.Value}}"
		{{end}}
		{{if .KubeConfigPath.Value}}
		kubeconfig_path = "{{.KubeConfigPath.Value}}"
		{{end}}
	}`))

	cases := []testAccResourceTemplate{}

	kindCluster := newLocalKindClusterStateV1()
	kindCluster.ID.Set("foo")
	kindCluster.Name.Set("acctestcluster")
	cases = append(cases, testAccResourceTemplate{
		"only name attribute",
		kindCluster,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_local_kind_cluster.foo", "id", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_local_kind_cluster.foo", "name", regexp.MustCompile(`^acctestcluster$`)),
		),
		false,
	})

	kindCluster2 := newLocalKindClusterStateV1()
	kindCluster2.ID.Set("foo")
	kindCluster2.Name.Set("acctestcluster")
	kindCluster2.Name.Set("/tmp/bologna/with/cheese/config")
	cases = append(cases, testAccResourceTemplate{
		"name and kubeconfig",
		kindCluster2,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_local_kind_cluster.foo", "id", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_local_kind_cluster.foo", "name", regexp.MustCompile(`^acctestcluster$`)),
			resource.TestMatchResourceAttr("enos_local_kind_cluster.foo", "kubeconfig_path", regexp.MustCompile(`^/tmp/bologna/with/cheese/config$`)),
		),
		false,
	})

	for _, test := range cases {
		t.Run(test.name, func(tt *testing.T) {
			buf := bytes.Buffer{}
			err := cfg.Execute(&buf, test.state)
			if err != nil {
				tt.Fatalf("error executing test template: %s", err.Error())
			}

			step := resource.TestStep{
				Config:             buf.String(),
				Check:              test.check,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			}

			resource.ParallelTest(tt, resource.TestCase{
				ProtoV6ProviderFactories: testProviders(t),
				Steps:                    []resource.TestStep{step},
			})
		})
	}
}

// TestResourceFileTransportInvalidAttributes ensures that we can gracefully
// handle invalid attributes in the transport configuration. Since it's a dynamic
// psuedo type we cannot rely on Terraform's built-in validation.
func TestResourceKindClusterInvalidAttributes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviders(t),
		Steps: []resource.TestStep{
			{
				Config:             `resource "enos_local_kind_cluster" "this" {}`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
				ExpectError:        regexp.MustCompile(`Missing required argument`),
			},
		},
	})
}

func TestClusterBuild(t *testing.T) {
	if _, accOk := os.LookupEnv("TF_ACC"); !accOk {
		t.Skip("Skipping test 'TestClusterBuild', because 'TF_ACC' not set")
	}

	checkDocker := exec.Command("docker", "ps")
	err := checkDocker.Run()
	if err != nil {
		t.Skip("Skipping test 'TestClusterBuild' since docker daemon not available")
	}

	kindClusterState := newLocalKindClusterStateV1()

	randomNum, err := rand.Int(rand.Reader, big.NewInt(1000000))
	require.NoError(t, err)
	name := randomNum.String()
	kindClusterState.Name.Set(name)

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Minute*3)
	t.Cleanup(cancelFunc)

	err = kindClusterState.createKindCluster(ctx)
	if err != nil {
		t.Fatalf("error executing cluster build test during create: %s", err.Error())
	}

	err = kindClusterState.readLocalKindCluster(ctx)
	if err != nil {
		t.Fatalf("error executing cluster build test during read: %s", err.Error())
	}

	err = kindClusterState.destroyLocalKindCluster(ctx)
	if err != nil {
		t.Fatalf("error executing cluster build test during destroy: %s", err.Error())
	}
}
