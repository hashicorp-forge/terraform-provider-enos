package nomad

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/test"
)

func Test_NomadTransport(t *testing.T) {
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

	host, ok := os.LookupEnv("ENOS_NOMAD_HOST")
	if !ok {
		t.Skip("Skipping test, since \"ENOS_NOMAD_HOST\" env var not specified")

		return nil
	}
	opts.Host = host

	allocationID, ok := os.LookupEnv("ENOS_NOMAD_ALLOCATION_ID")
	if !ok {
		t.Skip("Skipping test, since \"ENOS_NOMAD_ALLOCATION_ID\" env var not specified")

		return nil
	}
	opts.AllocationID = allocationID

	taskName, ok := os.LookupEnv("ENOS_NOMAD_TASK_NAME")
	if !ok {
		t.Skip("Skipping test, since \"ENOS_NOMAD_TASK_NAME\" env var not specified")
		return nil
	}
	opts.TaskName = taskName

	transport, err := NewTransport(opts)
	if err != nil {
		t.Fatal(err)
	}

	return transport
}
