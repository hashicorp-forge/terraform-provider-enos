// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"bytes"
	"crypto/rand"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kind/pkg/cluster"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/log"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/kind"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceKindCluster tests the kind_cluster resource.
func TestAccResourceKindClusterValidAttributes(t *testing.T) {
	t.Parallel()

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

	//nolint:paralleltest// because our resource handles it
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
// pseudo type we cannot rely on Terraform's built-in validation.
//
//nolint:paralleltest// because our resource handles it
func TestResourceKindClusterInvalidAttributes(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
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
	t.Parallel()

	if _, accOk := os.LookupEnv("TF_ACC"); !accOk {
		t.Skip("Skipping test 'TestClusterBuild', because 'TF_ACC' not set")
	}

	checkDocker := exec.Command("docker", "ps")
	err := checkDocker.Run()
	if err != nil {
		t.Skip("Skipping test 'TestClusterBuild' since docker daemon not available")
	}

	client := kind.NewLocalClient(log.NewNoopLogger())

	randomNum, err := rand.Int(rand.Reader, big.NewInt(1000000))
	require.NoError(t, err)
	name := randomNum.String()

	dir := t.TempDir()
	kubeConfigPath := filepath.Join(dir, "kubeconfig")

	info, err := client.CreateCluster(kind.CreateKindClusterRequest{Name: name, KubeConfigPath: kubeConfigPath})
	require.NoError(t, err)
	assert.NotNil(t, info)
	assert.NotEmpty(t, info.KubeConfigBase64)
	assert.Equal(t, "kind-"+name, info.ContextName)
	assert.NotEmpty(t, info.ClientCertificate)
	assert.NotEmpty(t, info.ClientKey)
	assert.NotEmpty(t, info.ClusterCACertificate)
	assert.NotEmpty(t, info.Endpoint)

	provider := cluster.NewProvider()
	nodes, err := provider.ListNodes(name)
	require.NoError(t, err)
	assert.Len(t, nodes, 1)

	require.NoError(t, client.DeleteCluster(kind.DeleteKindClusterRequest{Name: name, KubeConfigPath: kubeConfigPath}))
}
