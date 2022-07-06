package plugin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	kubeConfigEnvVar          = "KUBECONFIG"
	defaultClusterWaitTimeout = "30s"
)

type localKindCluster struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resourcerouter.Resource = (*localKindCluster)(nil)

type localKindClusterStateV1 struct {
	ID             *tfString
	Name           *tfString
	KubeConfigPath *tfString
	// amount of time to wait for the control plane to be ready, defaults to '30s'
	WaitTimeout          *tfString
	KubeConfigBase64     *tfString
	ContextName          *tfString
	ClientCertificate    *tfString
	ClientKey            *tfString
	ClusterCACertificate *tfString
	Endpoint             *tfString
}

var _ State = (*localKindClusterStateV1)(nil)

func newLocalKindCluster() *localKindCluster {
	return &localKindCluster{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newLocalKindClusterStateV1() *localKindClusterStateV1 {
	return &localKindClusterStateV1{
		ID:                   newTfString(),
		Name:                 newTfString(),
		KubeConfigPath:       newTfString(),
		WaitTimeout:          newTfString(),
		KubeConfigBase64:     newTfString(),
		ContextName:          newTfString(),
		ClientCertificate:    newTfString(),
		ClientKey:            newTfString(),
		ClusterCACertificate: newTfString(),
		Endpoint:             newTfString(),
	}
}

func (r *localKindCluster) Name() string {
	return "enos_local_kind_cluster"
}

func (r *localKindCluster) Schema() *tfprotov6.Schema {
	return newLocalKindClusterStateV1().Schema()
}

func (r *localKindCluster) SetProviderConfig(meta tftypes.Value) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.FromTerraform5Value(meta)
}

func (r *localKindCluster) GetProviderConfig() (*config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.Copy()
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (r *localKindCluster) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	transportUtil.UpgradeResourceState(ctx, newLocalKindClusterStateV1(), req, res)
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
func (r *localKindCluster) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	transportUtil.ImportResourceState(ctx, newLocalKindClusterStateV1(), req, res)
}

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (r *localKindCluster) ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	state := newLocalKindClusterStateV1()

	transportUtil.ValidateResourceConfig(ctx, state, req, res)
	if hasErrors(res.Diagnostics) {
		return
	}

	err := state.Validate(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
	}
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *localKindCluster) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	newState := newLocalKindClusterStateV1()

	err := unmarshal(newState, req.CurrentState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	tflog.Info(ctx, "Reading Local Kind Cluster", map[string]interface{}{
		"name":              newState.Name,
		"kubeconfig_base64": newState.KubeConfigBase64,
	})

	if err := newState.readLocalKindCluster(ctx); err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	// If the ID value is not set, the cluster was deleted outside of terraform, so we need to
	// marshall a nil value. This should cause a no-op.
	if newState.ID.Value() == "" {
		res.NewState, err = marshalDelete(newState)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		}
	} else {
		res.NewState, err = marshal(newState)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		}
	}
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *localKindCluster) PlanResourceChange(ctx context.Context, req tfprotov6.PlanResourceChangeRequest, res *tfprotov6.PlanResourceChangeResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	priorState := newLocalKindClusterStateV1()
	proposedState := newLocalKindClusterStateV1()

	err := unmarshal(priorState, req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	err = unmarshal(proposedState, req.ProposedNewState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	if _, ok := priorState.ID.Get(); !ok {
		proposedState.ID.Unknown = true
		proposedState.KubeConfigBase64.Unknown = true
		proposedState.ContextName.Unknown = true
		proposedState.ClientCertificate.Unknown = true
		proposedState.ClientKey.Unknown = true
		proposedState.ClusterCACertificate.Unknown = true
		proposedState.Endpoint.Unknown = true
	}

	res.RequiresReplace = []*tftypes.AttributePath{tftypes.NewAttributePathWithSteps([]tftypes.AttributePathStep{
		tftypes.AttributeName("name"),
	}), tftypes.NewAttributePathWithSteps([]tftypes.AttributePathStep{
		tftypes.AttributeName("kubeconfig_path"),
	})}

	res.PlannedState, err = marshal(proposedState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *localKindCluster) ApplyResourceChange(ctx context.Context, req tfprotov6.ApplyResourceChangeRequest, res *tfprotov6.ApplyResourceChangeResponse) {
	priorState := newLocalKindClusterStateV1()
	plannedState := newLocalKindClusterStateV1()

	err := unmarshal(plannedState, req.PlannedState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	err = unmarshal(priorState, req.PriorState)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}

	isDelete := plannedState.Name.Val == ""
	isCreate := priorState.ID.Val == ""
	isUpdate := !isDelete && !isCreate && reflect.DeepEqual(plannedState, priorState)

	switch {
	case isDelete:
		tflog.Debug(ctx, "Destroying a local kind cluster", map[string]interface{}{
			"name":            plannedState.Name.Val,
			"kubeconfig_path": plannedState.KubeConfigPath.Val,
		})

		if err := priorState.Validate(ctx); err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		}

		if err := priorState.destroyLocalKindCluster(ctx); err != nil {
			res.Diagnostics = append(res.Diagnostics, &tfprotov6.Diagnostic{
				Severity: tfprotov6.DiagnosticSeverityError,
				Summary:  "Failed to Destroy Kind Cluster",
				Detail:   err.Error(),
			})
			return
		}

		plannedState.ID.Set("")
		if res.NewState, err = marshalDelete(plannedState); err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		}
	case isCreate:
		tflog.Debug(ctx, "Create a local kind cluster", map[string]interface{}{
			"name":            plannedState.Name.Val,
			"kubeconfig_path": plannedState.KubeConfigPath.Val,
		})

		if err := plannedState.Validate(ctx); err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		}

		if err := plannedState.createKindCluster(ctx); err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return
		}
		if err := plannedState.readLocalKindCluster(ctx); err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return
		}
		plannedState.ID.Set(plannedState.Name.Val)

		if res.NewState, err = marshal(plannedState); err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		}
	case isUpdate:
		tflog.Warn(ctx, "Unexpected resource update for local kind cluster", map[string]interface{}{
			"name":            plannedState.Name.Val,
			"kubeconfig_path": plannedState.KubeConfigPath.Val,
		})
		res.Diagnostics = append(res.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityWarning,
			Summary:  "Unexpected Resource Update",
			Detail:   "Kind clusters cannot be updated in place.",
		})
	default:
		// this should never happen, but if it does let's just log a warning
		tflog.Warn(ctx, "Local kind cluster, unexpected apply state, state not one of create, delete or update.")
		res.Diagnostics = append(res.Diagnostics, &tfprotov6.Diagnostic{
			Severity: tfprotov6.DiagnosticSeverityWarning,
			Summary:  "Unexpected Resource Update",
			Detail:   "Unexpected apply state, state not one of create, delete or update.",
		})
	}
}

// Schema is the file states Terraform schema.
func (s *localKindClusterStateV1) Schema() *tfprotov6.Schema {
	return &tfprotov6.Schema{
		Version: 1,
		Block: &tfprotov6.SchemaBlock{
			Attributes: []*tfprotov6.SchemaAttribute{
				{
					Name:     "id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:        "name",
					Type:        tftypes.String,
					Description: "The name of the kind cluster",
					Required:    true,
				},
				{
					Name:        "kubeconfig_path",
					Description: "The path to the kubeconfig file",
					Type:        tftypes.String,
					Optional:    true,
				},
				{
					Name:        "wait_timeout",
					Description: "The amount of time to wait for the control plan to be ready, defaults to 30s",
					Type:        tftypes.String,
					Optional:    true,
				},
				{
					Name:        "kubeconfig_base64",
					Description: "Base64 encoded kubeconfig for the cluster",
					Type:        tftypes.String,
					Computed:    true,
					Sensitive:   true,
				},
				{
					Name:        "context_name",
					Description: "The name of the cluster context",
					Type:        tftypes.String,
					Computed:    true,
				},
				{
					Name:        "client_certificate",
					Description: "TLS client certificate for the cluster",
					Type:        tftypes.String,
					Computed:    true,
					Sensitive:   true,
				},
				{
					Name:        "client_key",
					Description: "TLS client key for the cluster",
					Type:        tftypes.String,
					Computed:    true,
					Sensitive:   true,
				},
				{
					Name:        "cluster_ca_certificate",
					Description: "TLS ca certificate for the cluster",
					Type:        tftypes.String,
					Computed:    true,
					Sensitive:   true,
				},
				{
					Name:        "endpoint",
					Description: "The url for the administration endpoint for the cluster",
					Type:        tftypes.String,
					Computed:    true,
				},
			},
		},
	}
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *localKindClusterStateV1) FromTerraform5Value(val tftypes.Value) error {
	if _, err := mapAttributesTo(val, map[string]interface{}{
		"id":                     s.ID,
		"name":                   s.Name,
		"kubeconfig_path":        s.KubeConfigPath,
		"wait_timeout":           s.WaitTimeout,
		"kubeconfig_base64":      s.KubeConfigBase64,
		"context_name":           s.ContextName,
		"client_certificate":     s.ClientCertificate,
		"client_key":             s.ClientKey,
		"cluster_ca_certificate": s.ClusterCACertificate,
		"endpoint":               s.Endpoint,
	}); err != nil {
		return wrapErrWithDiagnostics(err, "Error", "Failed to convert Terraform Value to kind cluster state.")
	}
	return nil
}

// Terraform5Type is the file state tftypes.Type.
func (s *localKindClusterStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":                     s.ID.TFType(),
		"name":                   s.Name.TFType(),
		"kubeconfig_path":        s.KubeConfigPath.TFType(),
		"wait_timeout":           s.WaitTimeout.TFType(),
		"kubeconfig_base64":      s.KubeConfigBase64.TFType(),
		"context_name":           s.ContextName.TFType(),
		"client_certificate":     s.ClientCertificate.TFType(),
		"client_key":             s.ClientKey.TFType(),
		"cluster_ca_certificate": s.ClusterCACertificate.TFType(),
		"endpoint":               s.Endpoint.TFType(),
	}}
}

// Terraform5Value is the file state tftypes.Value.
func (s *localKindClusterStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":                     s.ID.TFValue(),
		"name":                   s.Name.TFValue(),
		"kubeconfig_path":        s.KubeConfigPath.TFValue(),
		"wait_timeout":           s.WaitTimeout.TFValue(),
		"kubeconfig_base64":      s.KubeConfigBase64.TFValue(),
		"context_name":           s.ContextName.TFValue(),
		"client_certificate":     s.ClientCertificate.TFValue(),
		"client_key":             s.ClientKey.TFValue(),
		"cluster_ca_certificate": s.ClusterCACertificate.TFValue(),
		"endpoint":               s.Endpoint.TFValue(),
	})
}

// Validate validates the configuration. This will validate the source file
// exists and that the transport configuration is valid.
func (s *localKindClusterStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	kubeConfigPath, err := getKubeConfigPath(s.KubeConfigPath.Value())
	if err != nil {
		return wrapErrWithDiagnostics(err, "Validation Failure", "Failed to get a valid kubeconfig path", "kubeconfig_path")
	}

	// Check if the file exists
	if _, err = os.Stat(kubeConfigPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// if the file does not exist that's okay since, it will be created by kind when creating the
			// cluster
			return nil
		} else {
			return wrapErrWithDiagnostics(err, "Validation Error", "Failed to stat kubeconfig, this could be due to a permissions problem")
		}
	}

	// the file exists, so we need to check if it's a valid kubeconfig file.
	if _, err := clientcmd.LoadFromFile(kubeConfigPath); err != nil {
		return wrapErrWithDiagnostics(
			err,
			"Validation Failure",
			fmt.Sprintf("Failed to load existing kubeconfig file: [%s]", kubeConfigPath),
			"kubeconfig_path")
	}

	return nil
}

func (s *localKindClusterStateV1) createKindCluster(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Set variables necessary for cluster creation
	name, ok := s.Name.Get()
	if !ok {
		return fmt.Errorf("cannot create a cluster without cluster 'name'")
	}

	logFields := map[string]interface{}{"name": name}

	var copts []cluster.CreateOption
	kubeConfigPath, ok := s.KubeConfigPath.Get()
	if ok {
		path, err := getKubeConfigPath(kubeConfigPath)
		if err != nil {
			return fmt.Errorf("failed to get the kubeconfig path while creating a cluster, due to: %w", err)
		}
		copts = append(copts, cluster.CreateWithKubeconfigPath(path))
		logFields["kubeconfig_path"] = path
	}
	waitTimeout, ok := s.WaitTimeout.Get()
	if !ok {
		waitTimeout = defaultClusterWaitTimeout
	}

	wait, err := time.ParseDuration(waitTimeout)
	if err != nil {
		return fmt.Errorf("failed to parse 'wait_timeout': [%s], when creating cluster: [%s], due to: %w", waitTimeout, name, err)
	}

	copts = append(copts, cluster.CreateWithWaitForReady(wait))

	tflog.Info(ctx, "Creating Local Kind Cluster", logFields)
	provider := cluster.NewProvider(cluster.ProviderWithLogger(cmd.NewLogger()))

	if err := provider.Create(name, copts...); err != nil {
		return err
	}

	tflog.Info(ctx, "Local Kind Cluster Created", logFields)

	return nil
}

func (s *localKindClusterStateV1) readLocalKindCluster(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	name, ok := s.Name.Get()
	if !ok {
		return fmt.Errorf("encountered error reading kube config, missing 'name' attribute")
	}
	provider := cluster.NewProvider(cluster.ProviderWithLogger(cmd.NewLogger()))

	clusters, err := provider.List()
	if err != nil {
		return fmt.Errorf("failed to list clusters, due to: %w", err)
	}

	for _, clusterName := range clusters {
		// only try to read the cluster state if it exists. If it doesn't exist, it's been deleted
		// outside of terraform
		if clusterName == name {
			kconfig, err := provider.KubeConfig(name, false)
			if err != nil {
				return fmt.Errorf("encountered error reading kube config: %s", err)
			}

			encodedKubeConfig := base64.StdEncoding.EncodeToString([]byte(kconfig))
			s.KubeConfigBase64.Set(encodedKubeConfig)
			s.ContextName.Set("kind-" + s.Name.Value())

			config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kconfig))
			if err != nil {
				return fmt.Errorf("encountered error getting rest config: %s", err)
			}

			s.ClientCertificate.Set(string(config.CertData))
			s.ClientKey.Set(string(config.KeyData))
			s.ClusterCACertificate.Set(string(config.CAData))
			s.Endpoint.Set(config.Host)

			return nil
		}
	}

	// if we did not find a local kind cluster we need to set the state to unknown.
	clearValues(s.ID, s.Name, s.KubeConfigPath, s.ContextName, s.KubeConfigBase64, s.ClusterCACertificate, s.ClientCertificate, s.ClientKey, s.Endpoint)

	return nil
}

func clearValues(values ...*tfString) {
	for _, val := range values {
		val.Set("")
		val.Null = true
	}
}

func (s *localKindClusterStateV1) destroyLocalKindCluster(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	name, ok := s.Name.Get()
	if !ok {
		return fmt.Errorf("cannot delete cluster without cluster 'name'")
	}

	kubeconfigPath, err := getKubeConfigPath(s.KubeConfigPath.Value())
	if err != nil {
		return fmt.Errorf("failed to destroy cluster, due to: %w", err)
	}

	tflog.Info(ctx, "Destroying Local Kind Cluster", map[string]interface{}{
		"name":            name,
		"kubeconfig_path": kubeconfigPath,
	})

	provider := cluster.NewProvider(cluster.ProviderWithLogger(cmd.NewLogger()))

	if err := provider.Delete(name, kubeconfigPath); err != nil {
		return fmt.Errorf("failed to destroy cluster, due to: %w", err)
	}

	tflog.Info(ctx, "Local Kind Cluster Destroyed", map[string]interface{}{
		"name":            name,
		"kubeconfig_path": kubeconfigPath,
	})
	return nil
}

// TODO: move this to the kubernetes package when merged.
func getKubeConfigPath(kubeconfigPath string) (string, error) {
	if kubeconfigPath != "" {
		return kubeconfigPath, nil
	}

	kubeConfigEnv, ok := os.LookupEnv(kubeConfigEnvVar)
	if ok {
		list := filepath.SplitList(kubeConfigEnv)
		length := len(list)

		switch {
		case length == 0:
			return list[0], nil
		case length > 1:
			return "", fmt.Errorf("ambiguous kubeconfig path, using 'KUBECONFIG' env var value: [%s]", kubeConfigEnv)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home dir, when looking for the kubeconfig, due to: %w", err)
	}

	return filepath.Join(homeDir, ".kube", "config"), nil
}
