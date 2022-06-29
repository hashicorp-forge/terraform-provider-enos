package plugin

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/hashicorp/enos-provider/internal/remoteflight/boundary"
	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type boundaryInit struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resourcerouter.Resource = (*boundaryInit)(nil)

type boundaryInitStateV1 struct {
	ID         *tfString
	Transport  *embeddedTransportV1
	BinPath    *tfString
	ConfigPath *tfString
	// outputs
	AuthMethodID                 *tfString
	AuthMethodName               *tfString
	AuthLoginName                *tfString
	AuthPassword                 *tfString
	AuthScopeID                  *tfString
	AuthUserID                   *tfString
	AuthUserName                 *tfString
	HostCatalogID                *tfString
	HostSetID                    *tfString
	HostID                       *tfString
	HostType                     *tfString
	HostScopeID                  *tfString
	HostCatalogName              *tfString
	HostSetName                  *tfString
	HostName                     *tfString
	LoginRoleScopeID             *tfString
	LoginRoleName                *tfString
	OrgScopeID                   *tfString
	OrgScopeType                 *tfString
	OrgScopeName                 *tfString
	ProjectScopeID               *tfString
	ProjectScopeType             *tfString
	ProjectScopeName             *tfString
	TargetID                     *tfString
	TargetDefaultPort            *tfNum
	TargetSessionMaxSeconds      *tfNum
	TargetSessionConnectionLimit *tfNum
	TargetType                   *tfString
	TargetScopeID                *tfString
	TargetName                   *tfString
}

var _ State = (*boundaryInitStateV1)(nil)

func newBoundaryInit() *boundaryInit {
	return &boundaryInit{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newBoundaryInitStateV1() *boundaryInitStateV1 {
	return &boundaryInitStateV1{
		ID:         newTfString(),
		Transport:  newEmbeddedTransport(),
		BinPath:    newTfString(),
		ConfigPath: newTfString(),
		// outputs
		AuthMethodID:                 newTfString(),
		AuthMethodName:               newTfString(),
		AuthLoginName:                newTfString(),
		AuthPassword:                 newTfString(),
		AuthScopeID:                  newTfString(),
		AuthUserID:                   newTfString(),
		AuthUserName:                 newTfString(),
		HostCatalogID:                newTfString(),
		HostSetID:                    newTfString(),
		HostID:                       newTfString(),
		HostType:                     newTfString(),
		HostScopeID:                  newTfString(),
		HostCatalogName:              newTfString(),
		HostSetName:                  newTfString(),
		HostName:                     newTfString(),
		LoginRoleScopeID:             newTfString(),
		LoginRoleName:                newTfString(),
		OrgScopeID:                   newTfString(),
		OrgScopeType:                 newTfString(),
		OrgScopeName:                 newTfString(),
		ProjectScopeID:               newTfString(),
		ProjectScopeType:             newTfString(),
		ProjectScopeName:             newTfString(),
		TargetID:                     newTfString(),
		TargetDefaultPort:            newTfNum(),
		TargetSessionMaxSeconds:      newTfNum(),
		TargetSessionConnectionLimit: newTfNum(),
		TargetType:                   newTfString(),
		TargetScopeID:                newTfString(),
		TargetName:                   newTfString(),
	}
}

func (r *boundaryInit) Name() string {
	return "enos_boundary_init"
}

func (r *boundaryInit) Schema() *tfprotov6.Schema {
	return newBoundaryInitStateV1().Schema()
}

func (r *boundaryInit) SetProviderConfig(meta tftypes.Value) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.FromTerraform5Value(meta)
}

func (r *boundaryInit) GetProviderConfig() (*config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.Copy()
}

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (r *boundaryInit) ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	newState := newBoundaryInitStateV1()

	transportUtil.ValidateResourceConfig(ctx, newState, req, res)
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (r *boundaryInit) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	newState := newBoundaryInitStateV1()

	transportUtil.UpgradeResourceState(ctx, newState, req, res)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *boundaryInit) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	newState := newBoundaryInitStateV1()

	transportUtil.ReadResource(ctx, newState, req, res)
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
func (r *boundaryInit) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	newState := newBoundaryInitStateV1()

	transportUtil.ImportResourceState(ctx, newState, req, res)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *boundaryInit) PlanResourceChange(ctx context.Context, req tfprotov6.PlanResourceChangeRequest, res *tfprotov6.PlanResourceChangeResponse) {
	priorState := newBoundaryInitStateV1()
	proposedState := newBoundaryInitStateV1()

	transport := transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, r, req, res)
	if hasErrors(res.Diagnostics) {
		return
	}

	if _, ok := priorState.ID.Get(); !ok {
		proposedState.ID.Unknown = true
		proposedState.AuthMethodID.Unknown = true
		proposedState.AuthMethodName.Unknown = true
		proposedState.AuthLoginName.Unknown = true
		proposedState.AuthPassword.Unknown = true
		proposedState.AuthScopeID.Unknown = true
		proposedState.AuthUserID.Unknown = true
		proposedState.AuthUserName.Unknown = true
		proposedState.HostCatalogID.Unknown = true
		proposedState.HostSetID.Unknown = true
		proposedState.HostID.Unknown = true
		proposedState.HostType.Unknown = true
		proposedState.HostScopeID.Unknown = true
		proposedState.HostCatalogID.Unknown = true
		proposedState.HostSetName.Unknown = true
		proposedState.HostName.Unknown = true
		proposedState.LoginRoleScopeID.Unknown = true
		proposedState.LoginRoleName.Unknown = true
		proposedState.OrgScopeID.Unknown = true
		proposedState.OrgScopeType.Unknown = true
		proposedState.OrgScopeName.Unknown = true
		proposedState.ProjectScopeID.Unknown = true
		proposedState.ProjectScopeType.Unknown = true
		proposedState.ProjectScopeName.Unknown = true
		proposedState.TargetID.Unknown = true
		proposedState.TargetDefaultPort.Unknown = true
		proposedState.TargetSessionMaxSeconds.Unknown = true
		proposedState.TargetSessionConnectionLimit.Unknown = true
		proposedState.TargetType.Unknown = true
		proposedState.TargetScopeID.Unknown = true
		proposedState.TargetName.Unknown = true
	}

	transportUtil.PlanMarshalPlannedState(ctx, res, proposedState, transport)
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *boundaryInit) ApplyResourceChange(ctx context.Context, req tfprotov6.ApplyResourceChangeRequest, res *tfprotov6.ApplyResourceChangeResponse) {
	priorState := newBoundaryInitStateV1()
	plannedState := newBoundaryInitStateV1()

	transportUtil.ApplyUnmarshalState(ctx, priorState, plannedState, req, res)
	if hasErrors(res.Diagnostics) {
		return
	}

	_, okprior := priorState.ID.Get()
	_, okplan := plannedState.ID.Get()
	if okprior && !okplan {
		newState, err := marshalDelete(plannedState)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		} else {
			res.NewState = newState
		}
		return
	}

	transport := transportUtil.ApplyValidatePlannedAndBuildTransport(ctx, plannedState, r, res)
	if hasErrors(res.Diagnostics) {
		return
	}

	plannedID := "static"
	plannedState.ID.Set(plannedID)

	client, err := transport.Client(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return
	}
	defer client.Close() //nolint: staticcheck

	// If our priorState ID is blank then we're creating the resource
	if _, ok := priorState.ID.Get(); !ok {
		err = plannedState.Init(ctx, client)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return
		}
	} else if !reflect.DeepEqual(plannedState, priorState) {
		err = plannedState.Init(ctx, client)

		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(fmt.Errorf("%s", err)))
			return
		}
	}

	transportUtil.ApplyMarshalNewState(ctx, res, plannedState, transport)
}

// Schema is the file states Terraform schema.
func (s *boundaryInitStateV1) Schema() *tfprotov6.Schema {
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
					Name:     "bin_path",
					Type:     tftypes.String,
					Required: true,
				},
				{
					Name:     "config_path",
					Type:     tftypes.String,
					Required: true,
				},
				{
					Name:     "auth_method_id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "auth_method_name",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "auth_login_name",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "auth_password",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "auth_scope_id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "auth_user_id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "auth_user_name",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "host_catalog_id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "host_set_id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "host_id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "host_type",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "host_scope_id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "host_catalog_name",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "host_set_name",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "host_name",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "login_role_scope_id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "login_role_name",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "org_scope_id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "org_scope_type",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "org_scope_name",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "project_scope_id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "project_scope_type",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "project_scope_name",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "target_id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "target_default_port",
					Type:     tftypes.Number,
					Computed: true,
				},
				{
					Name:     "target_session_max_seconds",
					Type:     tftypes.Number,
					Computed: true,
				},
				{
					Name:     "target_session_connection_limit",
					Type:     tftypes.Number,
					Computed: true,
				},
				{
					Name:     "target_type",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "target_scope_id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "target_name",
					Type:     tftypes.String,
					Computed: true,
				},
				s.Transport.SchemaAttributeTransport(),
			},
		},
	}
}

// Validate validates the configuration. This will validate the source file
// exists and that the transport configuration is valid.
func (s *boundaryInitStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, ok := s.BinPath.Get(); !ok {
		return newErrWithDiagnostics("invalid configuration", "you must provide the Boundary bin path", "attribute")
	}

	if _, ok := s.ConfigPath.Get(); !ok {
		return newErrWithDiagnostics("invalid configuration", "you must provide the Boundary config path", "attribute")
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Boundary with As().
func (s *boundaryInitStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id":          s.ID,
		"bin_path":    s.BinPath,
		"config_path": s.ConfigPath,
		// outputs
		"auth_method_id":                  s.AuthMethodID,
		"auth_method_name":                s.AuthMethodName,
		"auth_login_name":                 s.AuthLoginName,
		"auth_password":                   s.AuthPassword,
		"auth_scope_id":                   s.AuthScopeID,
		"auth_user_id":                    s.AuthUserID,
		"auth_user_name":                  s.AuthUserName,
		"host_catalog_id":                 s.HostCatalogID,
		"host_set_id":                     s.HostSetID,
		"host_id":                         s.HostID,
		"host_type":                       s.HostType,
		"host_scope_id":                   s.HostScopeID,
		"host_catalog_name":               s.HostCatalogName,
		"host_set_name":                   s.HostSetName,
		"host_name":                       s.HostName,
		"login_role_scope_id":             s.LoginRoleScopeID,
		"login_role_name":                 s.LoginRoleName,
		"org_scope_id":                    s.OrgScopeID,
		"org_scope_type":                  s.OrgScopeType,
		"org_scope_name":                  s.OrgScopeName,
		"project_scope_id":                s.ProjectScopeID,
		"project_scope_type":              s.ProjectScopeType,
		"project_scope_name":              s.ProjectScopeName,
		"target_id":                       s.TargetID,
		"target_default_port":             s.TargetDefaultPort,
		"target_session_max_seconds":      s.TargetSessionMaxSeconds,
		"target_session_connection_limit": s.TargetSessionConnectionLimit,
		"target_type":                     s.TargetType,
		"target_scope_id":                 s.TargetScopeID,
		"target_name":                     s.TargetName,
	})
	if err != nil {
		return err
	}

	if !vals["transport"].IsKnown() {
		return nil
	}

	return s.Transport.FromTerraform5Value(vals["transport"])
}

// Terraform5Type is the file state tftypes.Type.
func (s *boundaryInitStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":          s.ID.TFType(),
		"transport":   s.Transport.Terraform5Type(),
		"bin_path":    s.BinPath.TFType(),
		"config_path": s.ConfigPath.TFType(),
		// outputs
		"auth_method_id":                  s.AuthMethodID.TFType(),
		"auth_method_name":                s.AuthMethodName.TFType(),
		"auth_login_name":                 s.AuthLoginName.TFType(),
		"auth_password":                   s.AuthPassword.TFType(),
		"auth_scope_id":                   s.AuthScopeID.TFType(),
		"auth_user_id":                    s.AuthUserID.TFType(),
		"auth_user_name":                  s.AuthUserName.TFType(),
		"host_catalog_id":                 s.HostCatalogID.TFType(),
		"host_set_id":                     s.HostSetID.TFType(),
		"host_id":                         s.HostID.TFType(),
		"host_type":                       s.HostType.TFType(),
		"host_scope_id":                   s.HostScopeID.TFType(),
		"host_catalog_name":               s.HostCatalogName.TFType(),
		"host_set_name":                   s.HostSetName.TFType(),
		"host_name":                       s.HostName.TFType(),
		"login_role_scope_id":             s.LoginRoleScopeID.TFType(),
		"login_role_name":                 s.LoginRoleName.TFType(),
		"org_scope_id":                    s.OrgScopeID.TFType(),
		"org_scope_type":                  s.OrgScopeType.TFType(),
		"org_scope_name":                  s.OrgScopeName.TFType(),
		"project_scope_id":                s.ProjectScopeID.TFType(),
		"project_scope_type":              s.ProjectScopeType.TFType(),
		"project_scope_name":              s.ProjectScopeName.TFType(),
		"target_id":                       s.TargetID.TFType(),
		"target_default_port":             s.TargetDefaultPort.TFType(),
		"target_session_max_seconds":      s.TargetSessionMaxSeconds.TFType(),
		"target_session_connection_limit": s.TargetSessionConnectionLimit.TFType(),
		"target_type":                     s.TargetType.TFType(),
		"target_scope_id":                 s.TargetScopeID.TFType(),
		"target_name":                     s.TargetName.TFType(),
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (s *boundaryInitStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":          s.ID.TFValue(),
		"transport":   s.Transport.Terraform5Value(),
		"bin_path":    s.BinPath.TFValue(),
		"config_path": s.ConfigPath.TFValue(),
		// outputs
		"auth_method_id":                  s.AuthMethodID.TFValue(),
		"auth_method_name":                s.AuthMethodName.TFValue(),
		"auth_login_name":                 s.AuthLoginName.TFValue(),
		"auth_password":                   s.AuthPassword.TFValue(),
		"auth_scope_id":                   s.AuthScopeID.TFValue(),
		"auth_user_id":                    s.AuthUserID.TFValue(),
		"auth_user_name":                  s.AuthUserName.TFValue(),
		"host_catalog_id":                 s.HostCatalogID.TFValue(),
		"host_set_id":                     s.HostSetID.TFValue(),
		"host_id":                         s.HostID.TFValue(),
		"host_type":                       s.HostType.TFValue(),
		"host_scope_id":                   s.HostScopeID.TFValue(),
		"host_catalog_name":               s.HostCatalogName.TFValue(),
		"host_set_name":                   s.HostSetName.TFValue(),
		"host_name":                       s.HostName.TFValue(),
		"login_role_scope_id":             s.LoginRoleScopeID.TFValue(),
		"login_role_name":                 s.LoginRoleName.TFValue(),
		"org_scope_id":                    s.OrgScopeID.TFValue(),
		"org_scope_type":                  s.OrgScopeType.TFValue(),
		"org_scope_name":                  s.OrgScopeName.TFValue(),
		"project_scope_id":                s.ProjectScopeID.TFValue(),
		"project_scope_type":              s.ProjectScopeType.TFValue(),
		"project_scope_name":              s.ProjectScopeName.TFValue(),
		"target_id":                       s.TargetID.TFValue(),
		"target_default_port":             s.TargetDefaultPort.TFValue(),
		"target_session_max_seconds":      s.TargetSessionMaxSeconds.TFValue(),
		"target_session_connection_limit": s.TargetSessionConnectionLimit.TFValue(),
		"target_type":                     s.TargetType.TFValue(),
		"target_scope_id":                 s.TargetScopeID.TFValue(),
		"target_name":                     s.TargetName.TFValue(),
	})
}

// EmbeddedTransport returns a pointer the resources embedded transport.
func (s *boundaryInitStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}

// Init initializes a Boundary cluster
func (s *boundaryInitStateV1) Init(ctx context.Context, client it.Transport) error {
	req := s.buildInitRequest()
	err := req.Validate()
	if err != nil {
		return wrapErrWithDiagnostics(err, "init", "could not validate config")
	}

	res, err := boundary.Init(ctx, client, req)
	if err != nil {
		return wrapErrWithDiagnostics(err, "init", "failed to init")
	}

	s.AuthMethodID.Set(res.AuthMethod.AuthMethodID)
	s.AuthMethodName.Set(res.AuthMethod.AuthMethodName)
	s.AuthLoginName.Set(res.AuthMethod.LoginName)
	s.AuthPassword.Set(res.AuthMethod.Password)
	s.AuthScopeID.Set(res.AuthMethod.ScopeID)
	s.AuthUserID.Set(res.AuthMethod.UserID)
	s.AuthUserName.Set(res.AuthMethod.UserName)
	s.HostCatalogID.Set(res.HostResources.HostCatalogID)
	s.HostSetID.Set(res.HostResources.HostSetID)
	s.HostID.Set(res.HostResources.HostID)
	s.HostType.Set(res.HostResources.Type)
	s.HostScopeID.Set(res.HostResources.ScopeID)
	s.HostCatalogID.Set(res.HostResources.HostCatalogID)
	s.HostSetName.Set(res.HostResources.HostSetName)
	s.HostName.Set(res.HostResources.HostName)
	s.LoginRoleScopeID.Set(res.LoginRole.ScopeID)
	s.LoginRoleName.Set(res.LoginRole.Name)
	s.OrgScopeID.Set(res.OrgScope.ScopeID)
	s.OrgScopeType.Set(res.OrgScope.Type)
	s.OrgScopeName.Set(res.OrgScope.Name)
	s.ProjectScopeID.Set(res.ProjectScope.ScopeID)
	s.ProjectScopeType.Set(res.ProjectScope.Type)
	s.ProjectScopeName.Set(res.ProjectScope.Name)
	s.TargetID.Set(res.Target.TargetID)
	s.TargetDefaultPort.Set(res.Target.DefaultPort)
	s.TargetSessionMaxSeconds.Set(res.Target.SessionMaxSeconds)
	s.TargetSessionConnectionLimit.Set(res.Target.SessionConnectionLimit)
	s.TargetType.Set(res.Target.Type)
	s.TargetScopeID.Set(res.Target.ScopeID)
	s.TargetName.Set(res.Target.Name)

	return err
}

// buildInitRequest builds an InitRequest with options set
func (s *boundaryInitStateV1) buildInitRequest() *boundary.InitRequest {
	opts := []boundary.InitRequestOpt{
		boundary.WithInitRequestBinPath(s.BinPath.Value()),
		boundary.WithInitRequestConfigPath(s.ConfigPath.Value()),
	}

	return boundary.NewInitRequest(opts...)
}
