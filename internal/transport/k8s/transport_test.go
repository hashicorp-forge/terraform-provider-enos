package k8s

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/hashicorp/enos-provider/internal/transport/test"

	it "github.com/hashicorp/enos-provider/internal/transport"
)

func Test_KubernetesTransport(t *testing.T) {
	t.Parallel()

	suite.Run(t, test.NewTransportTestSuite(func(t *testing.T) it.Transport {
		t.Helper()
		return createTransportOrSkipTest(t)
	}))
}

// createTransportOrSkipTest creates the transport or skips the test if any of the required options
// are missing.
func createTransportOrSkipTest(t *testing.T) it.Transport {
	t.Helper()
	opts := TransportOpts{}

	kubeConfig, ok := os.LookupEnv("ENOS_KUBECONFIG")
	if !ok {
		t.Skip("Skipping test, since \"ENOS_KUBECONFIG\" env var not specified")
		return nil
	}
	opts.KubeConfigBase64 = kubeConfig

	contextName, ok := os.LookupEnv("ENOS_K8S_CONTEXT_NAME")
	if !ok {
		t.Skip("Skipping test, since \"ENOS_K8S_CONTEXT_NAME\" env var not specified")
		return nil
	}
	opts.ContextName = contextName

	pod, ok := os.LookupEnv("ENOS_K8S_POD")
	if !ok {
		t.Skip("Skipping test, since \"ENOS_K8S_POD\" env var not specified")
		return nil
	}
	opts.Pod = pod

	if namespace, ok := os.LookupEnv("ENOS_K8S_NAMESPACE"); ok {
		opts.Namespace = namespace
	}

	if container, ok := os.LookupEnv("ENOS_K8S_CONTAINER"); ok {
		opts.Container = container
	}

	transport, err := NewTransport(opts)
	if err != nil {
		t.Fatal(err)
	}

	return transport
}
