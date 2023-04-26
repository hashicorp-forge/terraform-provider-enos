package plugin

import (
	"bytes"
	"context"
	"os"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/enos-provider/internal/server"
	dr "github.com/hashicorp/enos-provider/internal/server/datarouter"

	"github.com/hashicorp/enos-provider/internal/kubernetes"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

var emptyResult []kubernetes.PodInfo

// TestAccDataSourceArtifacotoryItem is an integration test that uses the
// actual HashiCorp artifactory service to resolve items based on the search
// criteria.
func TestAccDataSourceKubernetesPods(t *testing.T) {
	t.Parallel()

	_, okacc := os.LookupEnv("TF_ACC")

	if !okacc {
		t.Log(`skipping data "enos_kubernetes_pods" test because TF_ACC isn't set`)
		t.Skip()
		return
	}

	cfg := template.Must(template.New("enos_data_kubernetes_pods").Parse(`data "enos_kubernetes_pods" "bogus" {
  kubeconfig_base64  = "{{ .KubeConfigBase64.Value }}"
  context_name       = "{{ .ContextName.Value }}"
  namespace          = "{{ .Namespace.Value }}"
  {{ if  .ExpectedPodCount.Value -}}
  expected_pod_count = {{ .ExpectedPodCount.Value }}
  {{ end }}
  {{ if .WaitTimeout.Value }}
  wait_timeout       = "{{ .WaitTimeout.Value }}"
  {{ end }}
}

output "pods" {
  value = jsonencode([for podInfo in data.enos_kubernetes_pods.bogus.pods : "${podInfo.name}_${podInfo.namespace}_${jsonencode(podInfo.containers)}"])
}

output "transports_0_kubeconfig" {
  value = data.enos_kubernetes_pods.bogus.transports[0].kubeconfig_base64
}

output "transports_0_context_name" {
  value = data.enos_kubernetes_pods.bogus.transports[0].context_name
}

output "transports_0_namespace" {
  value = data.enos_kubernetes_pods.bogus.transports[0].namespace
}

output "transports_0_pod" {
  value = data.enos_kubernetes_pods.bogus.transports[0].pod
}

output "transports_0_container" {
  value = data.enos_kubernetes_pods.bogus.transports[0].container
}

output "transports_1_kubeconfig" {
  value = data.enos_kubernetes_pods.bogus.transports[1].kubeconfig_base64
}

output "transports_1_context_name" {
  value = data.enos_kubernetes_pods.bogus.transports[1].context_name
}

output "transports_1_namespace" {
  value = data.enos_kubernetes_pods.bogus.transports[1].namespace
}

output "transports_1_pod" {
  value = data.enos_kubernetes_pods.bogus.transports[1].pod
}

output "transports_1_container" {
  value = data.enos_kubernetes_pods.bogus.transports[1].container
}
`))

	fBytes, err := os.ReadFile("../fixtures/bogus_kubeconfig.b64")
	require.NoError(t, err)
	kubeConfig := string(fBytes)

	pods1 := []kubernetes.PodInfo{
		{
			Name:       "pod1",
			Namespace:  "yoyo",
			Containers: []string{"foo"},
		},
		{
			Name:       "pod2",
			Namespace:  "yoyo",
			Containers: []string{"bar"},
		},
	}

	state1 := newKubernetesPodStateV1()
	state1.KubeConfigBase64.Set(kubeConfig)
	state1.ContextName.Set("kind-bogus")
	state1.Namespace.Set("yoyo")
	state1.ExpectedPodCount.Set(2)
	state1.WaitTimeout.Set("2m")
	checkFunc1 := resource.ComposeTestCheckFunc(
		resource.TestMatchResourceAttr("data.enos_kubernetes_pods.bogus", "id", regexp.MustCompile(`^static$`)),
		resource.TestMatchResourceAttr("data.enos_kubernetes_pods.bogus", "kubeconfig_base64", regexp.MustCompile(kubeConfig)),
		resource.TestMatchResourceAttr("data.enos_kubernetes_pods.bogus", "context_name", regexp.MustCompile(`^kind-bogus$`)),
		resource.TestMatchResourceAttr("data.enos_kubernetes_pods.bogus", "namespace", regexp.MustCompile("yoyo")),
		resource.TestMatchResourceAttr("data.enos_kubernetes_pods.bogus", "expected_pod_count", regexp.MustCompile("2")),
		resource.TestMatchResourceAttr("data.enos_kubernetes_pods.bogus", "wait_timeout", regexp.MustCompile("2m")),
		resource.TestMatchResourceAttr("data.enos_kubernetes_pods.bogus", "context_name", regexp.MustCompile(`^kind-bogus$`)),
		resource.TestMatchOutput("pods", regexp.MustCompile(`.*pod1.*yoyo.*foo.*pod2.*yoyo.*bar.*`)),
		resource.TestCheckOutput("transports_0_kubeconfig", kubeConfig),
		resource.TestCheckOutput("transports_0_context_name", "kind-bogus"),
		resource.TestCheckOutput("transports_0_namespace", "yoyo"),
		resource.TestCheckOutput("transports_0_pod", "pod1"),
		resource.TestCheckOutput("transports_0_container", "foo"),
		resource.TestCheckOutput("transports_1_kubeconfig", kubeConfig),
		resource.TestCheckOutput("transports_1_context_name", "kind-bogus"),
		resource.TestCheckOutput("transports_1_namespace", "yoyo"),
		resource.TestCheckOutput("transports_1_pod", "pod2"),
		resource.TestCheckOutput("transports_1_container", "bar"),
	)

	state2 := newKubernetesPodStateV1()
	state2.KubeConfigBase64.Set(kubeConfig)
	state2.ContextName.Set("kind-not-present-context")
	state2.Namespace.Set("yoyo")
	notPresentError := regexp.MustCompile(`context: \[kind-not-present-context] not present`)

	state3 := newKubernetesPodStateV1()
	state3.KubeConfigBase64.Set("balogna")
	state3.ContextName.Set("some-context")
	invalidKubeConfigErr := regexp.MustCompile(`invalid kubeconfig`)

	for _, test := range []struct {
		name         string
		config       *kubernetesPodsStateV1
		queryResults []kubernetes.PodInfo
		checkFunc    resource.TestCheckFunc
		expectErr    *regexp.Regexp
	}{
		{"valid_config", state1, pods1, checkFunc1, nil},
		{"missing_context", state2, emptyResult, nil, notPresentError},
		{"invalid_kubeconfig", state3, emptyResult, nil, invalidKubeConfigErr},
	} {
		t.Run(test.name, func(tt *testing.T) {
			buf := bytes.Buffer{}
			require.NoError(t, cfg.Execute(&buf, test.config))

			step := resource.TestStep{
				Config: buf.String(),
				Check:  test.checkFunc,
			}

			if test.expectErr != nil {
				step.ExpectNonEmptyPlan = false
				step.ExpectError = test.expectErr
			} else {
				step.Check = checkFunc1
			}

			resource.Test(tt, resource.TestCase{
				ProtoV6ProviderFactories: testProvider(test.queryResults),
				Steps:                    []resource.TestStep{step},
			})
		})
	}
}

func testProvider(queryResults []kubernetes.PodInfo) map[string]func() (tfprotov6.ProviderServer, error) {
	ds := newKubernetesPods()
	ds.podInfoGetter = func(ctx context.Context, state kubernetesPodsStateV1) ([]kubernetes.PodInfo, error) {
		return queryResults, nil
	}
	s := server.New(
		server.RegisterProvider(newProvider()),
		server.RegisterDataRouter(dr.New(
			dr.RegisterDataSource(ds),
		)))

	return map[string]func() (tfprotov6.ProviderServer, error){
		"enos": func() (tfprotov6.ProviderServer, error) {
			return s, nil
		},
	}
}
