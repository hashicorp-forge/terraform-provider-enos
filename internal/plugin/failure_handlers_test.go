package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/enos-provider/internal/nomad"
	"github.com/hashicorp/nomad/api"

	"github.com/hashicorp/enos-provider/internal/transport/mock"

	"github.com/hashicorp/enos-provider/internal/log"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	"github.com/hashicorp/enos-provider/internal/remoteflight/systemd"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/hashicorp/enos-provider/internal/kubernetes"
	it "github.com/hashicorp/enos-provider/internal/transport"
)

type mockK8SClient struct {
	logs []byte
}

func (m *mockK8SClient) NewExecRequest(opts kubernetes.ExecRequestOpts) it.ExecRequest {
	// intentionally not implemented
	panic("implement me")
}

func (m *mockK8SClient) GetPodInfos(ctx context.Context, req kubernetes.GetPodInfoRequest) ([]kubernetes.PodInfo, error) {
	// intentionally not implemented
	panic("implement me")
}

func (m *mockK8SClient) GetLogs(ctx context.Context, req kubernetes.GetPodLogsRequest) (*kubernetes.GetPodLogsResponse, error) {
	return &kubernetes.GetPodLogsResponse{
		Namespace: req.Namespace,
		Pod:       req.Pod,
		Container: req.Container,
		Logs:      m.logs,
	}, nil
}

type mockSystemdClient struct {
	logs []byte
}

func (m mockSystemdClient) GetLogs(ctx context.Context, req systemd.GetLogsRequest) (remoteflight.GetLogsResponse, error) {
	return systemd.GetLogsResponse{
		Host: req.Host,
		Logs: m.logs,
	}, nil
}

func (m mockSystemdClient) CreateUnitFile(ctx context.Context, req *systemd.CreateUnitFileRequest) error {
	// intentionally not implemented
	panic("implement me")
}

func (m mockSystemdClient) ListServices(ctx context.Context) ([]systemd.ServiceInfo, error) {
	// intentionally not implemented
	panic("implement me")
}

func (m mockSystemdClient) RunSystemctlCommand(ctx context.Context, req *systemd.SystemctlCommandReq) (*systemd.SystemctlCommandRes, error) {
	// intentionally not implemented
	panic("implement me")
}

func (m mockSystemdClient) EnableService(ctx context.Context, unit string) error {
	// intentionally not implemented
	panic("implement me")
}

func (m mockSystemdClient) StartService(ctx context.Context, unit string) error {
	// intentionally not implemented
	panic("implement me")
}

func (m mockSystemdClient) StopService(ctx context.Context, unit string) error {
	// intentionally not implemented
	panic("implement me")
}

func (m mockSystemdClient) RestartService(ctx context.Context, unit string) error {
	// intentionally not implemented
	panic("implement me")
}

func (m mockSystemdClient) ServiceStatus(ctx context.Context, unit string) systemd.StatusCode {
	// intentionally not implemented
	panic("implement me")
}

func (m mockSystemdClient) IsActiveService(ctx context.Context, unit string) bool {
	// intentionally not implemented
	panic("implement me")
}

type mockNomadClient struct {
	logs       []byte
	namespace  string
	allocation string
}

func (m mockNomadClient) GetAllocation(allocPrefix string) (*api.Allocation, error) {
	// intentionally not implemented
	panic("implement me")
}

func (m mockNomadClient) GetAllocationInfo(allocID string) (*api.Allocation, error) {
	// intentionally not implemented
	panic("implement me")
}

func (m mockNomadClient) NewExecRequest(opts nomad.ExecRequestOpts) it.ExecRequest {
	// intentionally not implemented
	panic("implement me")
}

func (m mockNomadClient) Exec(ctx context.Context, opts nomad.ExecRequestOpts, streams *it.ExecStreams) *it.ExecResponse {
	// intentionally not implemented
	panic("implement me")
}

func (m mockNomadClient) GetLogs(ctx context.Context, req nomad.GetTaskLogsRequest) (*nomad.GetTaskLogsResponse, error) {
	return &nomad.GetTaskLogsResponse{
		Namespace:  m.namespace,
		Allocation: m.allocation,
		Task:       req.Task,
		Logs:       m.logs,
	}, nil
}

func (m mockNomadClient) Close() {
	// do nothing on purpose
}

func TestTransportDebugFailureHandler(t *testing.T) {
	t.Parallel()

	type test struct {
		name      string
		transport transportState
		want      map[string]string //key value map transport output to diagnostic
	}

	var tests []test

	transportSSH := newEmbeddedTransportSSH()
	transportSSH.Host.Set("10.0.0.1")
	transportSSH.User.Set("ubuntu")
	transportSSH.PrivateKeyPath.Set("/some/path/to/the/private/key.pem")
	transportSSH.PrivateKey.Set("This is a fake private key")
	transportSSH.PassphrasePath.Set("/this/is/a/passphrase/path")
	transportSSH.Passphrase.Set("123456")

	tests = append(tests, test{
		name:      "SSH_transport_all_values",
		transport: transportSSH,
		want: map[string]string{
			"host":             "10.0.0.1",
			"user":             "ubuntu",
			"private_key_path": "/some/path/to/the/private/key.pem",
			"private_key":      "This is a fake private key",
			"passphrase_path":  "/this/is/a/passphrase/path",
			"passphrase":       "[redacted]",
		},
	})

	transportSSH2 := newEmbeddedTransportSSH()
	transportSSH2.Host.Set("10.0.0.1")
	transportSSH2.User.Set("ubuntu")
	transportSSH2.PrivateKey.Set("This is a fake private key")
	transportSSH2.PassphrasePath.Set("/this/is/a/passphrase/path")
	transportSSH2.Passphrase.Set("123456")

	tests = append(tests, test{
		name:      "SSH_transport_one_empty",
		transport: transportSSH2,
		want: map[string]string{
			"host":             "10.0.0.1",
			"user":             "ubuntu",
			"private_key_path": "null",
			"private_key":      "This is a fake private key",
			"passphrase_path":  "/this/is/a/passphrase/path",
			"passphrase":       "[redacted]",
		},
	})

	transportK8S := newEmbeddedTransportK8Sv1()
	transportK8S.KubeConfigBase64.Set("some kubeconfig")
	transportK8S.ContextName.Set("some context")
	transportK8S.Namespace.Set("namespace")
	transportK8S.Pod.Set("some pod")
	transportK8S.Container.Set("some container")

	tests = append(tests, test{
		name:      "K8S_transport_all_values",
		transport: transportK8S,
		want: map[string]string{
			"kubeconfig_base64": "[redacted]",
			"context_name":      "some context",
			"namespace":         "namespace",
			"pod":               "some pod",
			"container":         "some container",
		},
	})

	transportK8S2 := newEmbeddedTransportK8Sv1()
	transportK8S2.KubeConfigBase64.Set("some kubeconfig")
	transportK8S2.ContextName.Set("some context")
	transportK8S2.Pod.Set("some pod")
	transportK8S2.Container.Set("some container")

	tests = append(tests, test{
		name:      "K8S_transport_one_empty",
		transport: transportK8S2,
		want: map[string]string{
			"kubeconfig_base64": "[redacted]",
			"context_name":      "some context",
			"namespace":         "null",
			"pod":               "some pod",
			"container":         "some container",
		},
	})

	transportNomad := newEmbeddedTransportNomadv1()
	transportNomad.Host.Set("10.0.0.1")
	transportNomad.SecretID.Set("some secret")
	transportNomad.AllocationID.Set("ed23c5")
	transportNomad.TaskName.Set("some task")

	tests = append(tests, test{
		name:      "Nomad_transport_all_values",
		transport: transportNomad,
		want: map[string]string{
			"host":          "10.0.0.1",
			"secret_id":     "[redacted]",
			"allocation_id": "ed23c5",
			"task_name":     "some task",
		},
	})

	transportNomad2 := newEmbeddedTransportNomadv1()
	transportNomad2.Host.Set("10.0.0.1")
	transportNomad2.AllocationID.Set("ed23c5")
	transportNomad2.TaskName.Set("some task")

	tests = append(tests, test{
		name:      "Nomad_transport_one_empty",
		transport: transportNomad2,
		want: map[string]string{
			"host":          "10.0.0.1",
			"secret_id":     "null",
			"allocation_id": "ed23c5",
			"task_name":     "some task",
		},
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := newEmbeddedTransport()
			transport.resolvedTransport = tt.transport
			handler := TransportDebugFailureHandler(transport)

			errDiag := &tfprotov6.Diagnostic{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "Failure",
				Detail:   "This thing failed",
			}

			handler(context.Background(), errDiag, tftypes.NewValue(tftypes.String, ""))

			for key, value := range tt.want {
				expected := fmt.Sprintf("%s : %s", key, value)
				assert.Contains(t, errDiag.Detail, expected)
			}
		})
	}
}

func TestGetLogsFailureHandler(t *testing.T) {
	t.Parallel()

	existDir := t.TempDir()

	noExistDir := t.TempDir()
	assert.NoError(t, os.RemoveAll(noExistDir))

	logs := []byte(`Preparing to make tacos
Found cheese
Found Tortillas
Found Beans
Error: Failed to find chicken
Taco Failed`)

	k8sTransport := newEmbeddedTransportK8Sv1()
	k8sTransport.Pod.Set("tacos")
	k8sTransport.Namespace.Set("food")
	k8sTransport.Container.Set("chicken")
	k8sTransport.KubeConfigBase64.Set("bogus")
	k8sTransport.ContextName.Set("taco_cluster")
	k8sTransport.k8sClientFactory = func(cfg kubernetes.ClientCfg) (kubernetes.Client, error) {
		return &mockK8SClient{logs: logs}, nil
	}

	sshTransport := newEmbeddedTransportSSH()
	sshTransport.Host.Set("10.0.0.1")
	sshTransport.User.Set("ubuntu")
	sshTransport.PrivateKeyPath.Set("/some/private/key/path/key.pem")
	sshTransport.sshTransportBuilder = func(state *embeddedTransportSSHv1, ctx context.Context) (it.Transport, error) {
		return mock.New(), nil
	}
	sshTransport.systemdClientFactory = func(transport it.Transport, logger log.Logger) systemd.Client {
		return &mockSystemdClient{logs: logs}
	}

	nomadTransport := newEmbeddedTransportNomadv1()
	nomadTransport.TaskName.Set("chicken")
	nomadTransport.nomadClientFactory = func(cfg nomad.ClientCfg) (nomad.Client, error) {
		return &mockNomadClient{
			logs:       logs,
			namespace:  "food",
			allocation: "tacos",
		}, nil
	}

	tests := []struct {
		name                string
		dir                 string
		transport           transportState
		expectedLogFileName string
	}{
		{
			name:                "dir_exists",
			dir:                 existDir,
			transport:           k8sTransport,
			expectedLogFileName: "taco-truck_food_tacos_chicken.log",
		},
		{
			name:                "dir_not_exists",
			dir:                 noExistDir,
			transport:           sshTransport,
			expectedLogFileName: "taco-truck_10.0.0.1.log",
		},
		{
			name:                "nomad_transport",
			dir:                 existDir,
			transport:           nomadTransport,
			expectedLogFileName: "taco-truck_food_tacos_chicken.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embeddedTransport := newEmbeddedTransport()
			embeddedTransport.resolvedTransport = tt.transport

			handler := GetApplicationLogsFailureHandler(embeddedTransport, "taco-truck")

			diag := &tfprotov6.Diagnostic{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "Taco Failure",
				Detail:   "Failed to make taco since there was no chicken.",
			}
			providerConfig := newProviderConfig()
			providerConfig.DebugDataRootDir.Set(tt.dir)

			handler(context.Background(), diag, providerConfig.Terraform5Value())

			logFile := filepath.Join(tt.dir, tt.expectedLogFileName)

			assert.FileExists(t, logFile)

			logContents, err := os.ReadFile(logFile)
			assert.NoError(t, err)
			assert.Equal(t, logs, logContents)

			assert.Equal(t, fmt.Sprintf(`Failed to make taco since there was no chicken.

Application Logs:
  taco-truck: %s`, logFile), diag.Detail)
		})
	}
}

func TestGetLogsFailureHandlerNotConfigured(t *testing.T) {
	dir := t.TempDir()
	logs := []byte(`Preparing to make tacos
Found cheese
Found Tortillas
Found Beans
Error: Failed to find chicken
Taco Failed`)

	k8sTransport := newEmbeddedTransportK8Sv1()
	k8sTransport.Pod.Set("tacos")
	k8sTransport.Namespace.Set("food")
	k8sTransport.Container.Set("chicken")
	k8sTransport.KubeConfigBase64.Set("bogus")
	k8sTransport.ContextName.Set("taco_cluster")
	k8sTransport.k8sClientFactory = func(cfg kubernetes.ClientCfg) (kubernetes.Client, error) {
		return &mockK8SClient{logs: logs}, nil
	}
	embeddedTransport := newEmbeddedTransport()
	embeddedTransport.resolvedTransport = k8sTransport

	handler := GetApplicationLogsFailureHandler(embeddedTransport, "taco-truck")

	diag := &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityError,
		Summary:  "Taco Failure",
		Detail:   "Failed to make taco since there was no chicken.",
	}
	providerConfig := newProviderConfig()

	handler(context.Background(), diag, providerConfig.Terraform5Value())

	logFile := filepath.Join(dir, "taco-truck_food_tacos_chicken.log")

	assert.NoFileExists(t, logFile)

	assert.Equal(t, "Failed to make taco since there was no chicken.", diag.Detail)
}
