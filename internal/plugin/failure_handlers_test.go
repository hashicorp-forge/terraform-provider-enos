package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/enos-provider/internal/kubernetes"
	"github.com/hashicorp/enos-provider/internal/log"
	"github.com/hashicorp/enos-provider/internal/nomad"
	"github.com/hashicorp/enos-provider/internal/remoteflight"
	"github.com/hashicorp/enos-provider/internal/remoteflight/systemd"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/mock"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type mockSystemdClient struct {
	// logs is a map of unit name to logs
	logs map[string][]byte
}

func (m mockSystemdClient) GetUnitJournal(ctx context.Context, req *systemd.GetUnitJournalRequest) (remoteflight.GetLogsResponse, error) {
	logs, ok := m.logs[req.Unit]
	if !ok {
		return nil, fmt.Errorf("unit not installed")
	}

	return &systemd.GetUnitJournalResponse{
		Unit: req.Unit,
		Host: req.Host,
		Logs: logs,
	}, nil
}

func (m mockSystemdClient) CreateUnitFile(ctx context.Context, req *systemd.CreateUnitFileRequest) error {
	// intentionally not implemented
	panic("implement me")
}

func (m mockSystemdClient) ListServices(ctx context.Context) ([]systemd.ServiceInfo, error) {
	return []systemd.ServiceInfo{
		{
			Unit:        "fish",
			Load:        "loaded",
			Active:      "active",
			Sub:         "exited",
			Description: "fish taco",
		},
		{
			Unit:        "consul",
			Load:        "loaded",
			Active:      "active",
			Sub:         "running",
			Description: "consul service",
		},
		{
			Unit:        "chicken",
			Load:        "loaded",
			Active:      "active",
			Sub:         "running",
			Description: "chicken taco",
		},
	}, nil
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

func (m mockSystemdClient) ServiceStatus(ctx context.Context, unit string) systemd.SystemctlStatusCode {
	// intentionally not implemented
	panic("implement me")
}

func (m mockSystemdClient) ShowProperties(ctx context.Context, unit string) (systemd.UnitProperties, error) {
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
		want      map[string]string // key value map transport output to diagnostic
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
	require.NoError(t, os.RemoveAll(noExistDir))

	chickenLogs := []byte(`Preparing to make tacos
Found cheese
Found Tortillas
Found Beans
Error: Failed to find chicken
Taco Failed`)

	consulLogs := []byte(`Preparing to run consul
Found vault
Found members
Error: Failed to find consul`)

	k8sTransport := newEmbeddedTransportK8Sv1()
	k8sTransport.Pod.Set("tacos")
	k8sTransport.Namespace.Set("food")
	k8sTransport.Container.Set("chicken")
	k8sTransport.KubeConfigBase64.Set("bogus")
	k8sTransport.ContextName.Set("taco_cluster")
	k8sTransport.k8sClientFactory = func(cfg kubernetes.ClientCfg) (kubernetes.Client, error) {
		return &kubernetes.MockClient{
			GetLogsFunc: kubernetes.NewMockGetLogsFunc(chickenLogs),
		}, nil
	}

	sshTransport := newEmbeddedTransportSSH()
	sshTransport.Host.Set("10.0.0.1")
	sshTransport.User.Set("ubuntu")
	sshTransport.PrivateKeyPath.Set("/some/private/key/path/key.pem")
	sshTransport.sshTransportBuilder = func(state *embeddedTransportSSHv1, ctx context.Context) (it.Transport, error) {
		return mock.New(), nil
	}
	serviceMap := map[string][]byte{
		"chicken": chickenLogs,
		"consul":  consulLogs,
	}
	sshTransport.systemdClientFactory = func(transport it.Transport, logger log.Logger) systemd.Client {
		return &mockSystemdClient{logs: serviceMap}
	}

	nomadTransport := newEmbeddedTransportNomadv1()
	nomadTransport.TaskName.Set("chicken")
	nomadTransport.nomadClientFactory = func(cfg nomad.ClientCfg) (nomad.Client, error) {
		return &mockNomadClient{
			logs:       chickenLogs,
			namespace:  "food",
			allocation: "tacos",
		}, nil
	}

	tests := []struct {
		name                 string
		dir                  string
		transport            transportState
		expectedLogFileNames map[string]string
		expectedServices     []string
	}{
		{
			name:      "dir_exists",
			dir:       existDir,
			transport: k8sTransport,
			expectedLogFileNames: map[string]string{
				"chicken": "taco_cluster_food_tacos_chicken.log",
			},
			expectedServices: []string{"chicken"},
		},
		{
			name:      "dir_not_exists",
			dir:       noExistDir,
			transport: sshTransport,
			expectedLogFileNames: map[string]string{
				"chicken": "chicken_10.0.0.1.log",
				"consul":  "consul_10.0.0.1.log",
			},
			expectedServices: []string{"consul", "chicken"},
		},
		{
			name:      "nomad_transport",
			dir:       existDir,
			transport: nomadTransport,
			expectedLogFileNames: map[string]string{
				"chicken": "food_tacos_chicken.log",
			},
			expectedServices: []string{"chicken"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			embeddedTransport := newEmbeddedTransport()
			embeddedTransport.resolvedTransport = tt.transport
			services := []string{"chicken", "sshd", "fish"}

			handler := GetApplicationLogsFailureHandler(embeddedTransport, services)

			diag := &tfprotov6.Diagnostic{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "Taco Failure",
				Detail:   "Failed to make taco since there was no chicken.",
			}
			providerConfig := newProviderConfig()
			providerConfig.DebugDataRootDir.Set(tt.dir)

			handler(context.Background(), diag, providerConfig.Terraform5Value())

			for _, service := range tt.expectedServices {
				logFile := filepath.Join(tt.dir, tt.expectedLogFileNames[service])

				assert.FileExists(t, logFile)

				logContents, err := os.ReadFile(logFile)
				require.NoError(t, err)
				assert.Equal(t, serviceMap[service], logContents)
				assert.Contains(t, diag.Detail, fmt.Sprintf("  %s: %s", service, logFile))
			}

			assert.Contains(t, diag.Detail, `Failed to make taco since there was no chicken.

Application Logs:`)
		})
	}
}

func TestGetLogsFailureHandlerNotConfigured(t *testing.T) {
	t.Parallel()
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
		return &kubernetes.MockClient{
			GetLogsFunc: kubernetes.NewMockGetLogsFunc(logs),
		}, nil
	}
	embeddedTransport := newEmbeddedTransport()
	embeddedTransport.resolvedTransport = k8sTransport

	handler := GetApplicationLogsFailureHandler(embeddedTransport, []string{"taco-truck"})

	diag := &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityError,
		Summary:  "Taco Failure",
		Detail:   "Failed to make taco since there was no chicken.",
	}
	providerConfig := newProviderConfig()

	handler(context.Background(), diag, providerConfig.Terraform5Value())

	logFile := filepath.Join(dir, "taco_cluster_food_tacos_chicken.log")

	assert.NoFileExists(t, logFile)

	assert.Equal(t, "Failed to make taco since there was no chicken.", diag.Detail)
}
