package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/enos-provider/internal/kubernetes"
	"github.com/hashicorp/enos-provider/internal/log"
	"github.com/hashicorp/enos-provider/internal/nomad"
	"github.com/hashicorp/enos-provider/internal/remoteflight"
	"github.com/hashicorp/enos-provider/internal/remoteflight/systemd"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// FailureHandler A function that can can be used to handle state plan/apply failures and enhance
// the error diagnostic
type FailureHandler func(ctx context.Context, errDiag *tfprotov6.Diagnostic, providerConfig tftypes.Value)

// failureHandlers simple wrapper for a slice of failure handlers, can be used to chain the execution
// of multiple failure handlers.
type failureHandlers []FailureHandler

func newFailureHandlers(handlers ...FailureHandler) failureHandlers {
	return handlers
}

// HandleFailure cycles through all the failure handlers and handles a failure
func (f failureHandlers) HandleFailure(ctx context.Context, errDiag *tfprotov6.Diagnostic, providerConfig tftypes.Value) {
	for _, h := range f {
		h(ctx, errDiag, providerConfig)
	}
}

// TransportDebugFailureHandler adds the transport configuration to the provided diagnostic
func TransportDebugFailureHandler(et *embeddedTransportV1) FailureHandler {
	return func(ctx context.Context, errDiag *tfprotov6.Diagnostic, providerConfig tftypes.Value) {
		errDiag.Detail = fmt.Sprintf("%s\n\n%s", errDiag.Detail, et.Debug())
	}
}

// GetApplicationLogsFailureHandler Creates a failure handler that fetches application logs, downloads them
// to a file and updates the error diagnostic with a list of logs that were retrieved and where they
// were saved.
func GetApplicationLogsFailureHandler(et *embeddedTransportV1, appName string) FailureHandler {
	return func(ctx context.Context, errDiag *tfprotov6.Diagnostic, providerConfig tftypes.Value) {
		logger := log.NewLogger(ctx)
		logger = logger.WithValues(map[string]interface{}{
			"app_name": appName,
		})

		cfg := newProviderConfig()
		if err := cfg.FromTerraform5Value(providerConfig); err != nil {
			logger.Error("failed to get data dir", map[string]interface{}{
				"error": err,
			})
			return
		}

		dataDir, ok := cfg.DebugDataRootDir.Get()
		if !ok {
			logger.Debug("skipped Logs Failure Handler, since a diagnostics data dir was not configured")
			return
		}

		var resp remoteflight.GetLogsResponse
		var err error
		switch transport := et.resolvedTransport.(type) {
		case *embeddedTransportSSHv1:
			logger = logger.WithValues(map[string]interface{}{
				"user": transport.User.Val,
				"host": transport.Host.Val,
			})
			logger.Info("Attempting to gather systemd logs")
			resp, err = getSystemdLogs(ctx, logger, transport, systemd.GetLogsRequest{
				Unit: appName,
				Host: transport.Host.Val,
			})
		case *embeddedTransportNomadv1:
			logger = logger.WithValues(map[string]interface{}{
				"allocation_id": transport.AllocationID.Val,
				"task":          transport.TaskName.Val,
				"host":          transport.Host.Val,
			})
			logger.Info("Attempting to gather Nomad task logs")
			resp, err = getNomadLogs(ctx, transport)
		case *embeddedTransportK8Sv1:
			logger = logger.WithValues(map[string]interface{}{
				"context_name": transport.ContextName.Val,
				"namespace":    transport.Namespace.Val,
				"pod":          transport.Pod.Val,
				"container":    transport.Container.Val,
			})
			logger.Info("Attempting to gather Kubernetes pod logs")
			resp, err = getK8sLogs(ctx, transport)
		default:
			logger.Error("failed to get logs, unknown transport type", map[string]interface{}{
				"transport_type": transport.Type().String(),
			})
			return
		}

		if err != nil {
			logger.Error("failed to get logs", map[string]interface{}{
				"error": err,
			})
			return
		}

		logFile := filepath.Join(dataDir, resp.GetLogFileName(appName))
		logger = logger.With("log_file", logFile)

		logger.Info("Got logs, writing to a file")

		if err := saveLogsToFile(logFile, resp.GetLogs()); err != nil {
			logger.Error("failed to save logs to file", map[string]interface{}{
				"error": err,
			})
			return
		}

		errDiag.Detail = fmt.Sprintf("%s\n\nApplication Logs:\n%s", errDiag.Detail, fmt.Sprintf("  %s: %s", appName, logFile))
		logger.Info("wrote log file location to diagnostic")
	}
}

func saveLogsToFile(logFile string, logs []byte) error {
	err := os.MkdirAll(filepath.Dir(logFile), 0o750)
	if err != nil {
		return fmt.Errorf("failed to create folder for log file: [%s], due to: %w", logFile, err)
	}
	err = os.WriteFile(logFile, logs, 0o640)
	if err != nil {
		return fmt.Errorf("failed to write logfile: [%s], due to: %w", logFile, err)
	}

	return nil
}

func getK8sLogs(ctx context.Context, transport *embeddedTransportK8Sv1) (*kubernetes.GetPodLogsResponse, error) {
	pod, ok := transport.Pod.Get()
	if !ok {
		return nil, fmt.Errorf("missing [pod] property, cannot fetch logs")
	}

	client, err := transport.k8sClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client, due to: %w", err)
	}

	req := kubernetes.GetPodLogsRequest{
		Pod: pod,
	}

	if namespace, ok := transport.Namespace.Get(); ok {
		req.Namespace = namespace
	}
	if container, ok := transport.Container.Get(); ok {
		req.Container = container
	}

	resp, err := client.GetLogs(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs for pod: [%s], due to:%w", pod, err)
	}

	return resp, nil
}

func getSystemdLogs(ctx context.Context, logger log.Logger, transport *embeddedTransportSSHv1, req systemd.GetLogsRequest) (remoteflight.GetLogsResponse, error) {
	sysd, err := transport.systemdClient(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create systemd client, due to: %w", err)
	}

	return sysd.GetLogs(ctx, req)
}

func getNomadLogs(ctx context.Context, transport *embeddedTransportNomadv1) (*nomad.GetTaskLogsResponse, error) {
	client, err := transport.nomadClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	return client.GetLogs(ctx, nomad.GetTaskLogsRequest{
		AllocationID: transport.AllocationID.Val,
		Task:         transport.TaskName.Val,
	})
}
