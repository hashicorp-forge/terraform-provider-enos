package plugin

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/enos-provider/internal/diags"
	"github.com/hashicorp/enos-provider/internal/remoteflight"
	resource "github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/enos-provider/internal/server/state"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type user struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resource.Resource = (*user)(nil)

type userStateV1 struct {
	ID      *tfString
	Name    *tfString
	HomeDir *tfString
	Shell   *tfString
	GID     *tfString
	UID     *tfString

	Transport *embeddedTransportV1

	failureHandlers
}

var _ state.State = (*userStateV1)(nil)

func newUser() *user {
	return &user{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newUserStateV1() *userStateV1 {
	transport := newEmbeddedTransport()
	fh := failureHandlers{TransportDebugFailureHandler(transport)}

	return &userStateV1{
		ID:              newTfString(),
		Name:            newTfString(),
		HomeDir:         newTfString(),
		Shell:           newTfString(),
		GID:             newTfString(),
		UID:             newTfString(),
		Transport:       transport,
		failureHandlers: fh,
	}
}

func (f *user) Name() string {
	return "enos_user"
}

func (f *user) Schema() *tfprotov6.Schema {
	return newUserStateV1().Schema()
}

func (f *user) SetProviderConfig(providerConfig tftypes.Value) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.providerConfig.FromTerraform5Value(providerConfig)
}

func (f *user) GetProviderConfig() (*config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.providerConfig.Copy()
}

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (f *user) ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	newState := newUserStateV1()

	transportUtil.ValidateResourceConfig(ctx, newState, req, res)
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (f *user) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	newState := newUserStateV1()

	transportUtil.UpgradeResourceState(ctx, newState, req, res)
}

// ReadResource is the request Terraform sends when it wants to get the latest state for the resource.
// We'll exit gracefully if we're unable to read the resource since it's possible that it does not
// yet exist.
func (f *user) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	currentState := newUserStateV1()

	// Make sure we marshal our new state when we return
	defer func() {
		var err error
		res.NewState, err = state.Marshal(currentState)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Serialization Error", err))
		}
	}()

	// Try and build a client to read the resource. If we encounter missing preconditions or errors
	// then we'll return our state unmodified, eg: whatever was passed in and Unknown values for our
	// optional and computed attributes.
	transport := transportUtil.ReadUnmarshalAndBuildTransport(ctx, currentState, f, req, res)
	if transport == nil {
		return
	}

	name, ok := currentState.Name.Get()
	if !ok {
		return
	}

	it, err := transport.Client(ctx)
	if err != nil {
		return
	}

	// We actually have a valid transport and user name. Try and find the user.
	user, err := remoteflight.FindUser(ctx, it, name)
	if err != nil {
		// We couldn't find our user. Set all of our current state attrs to Unknown since we can't read
		// it and need to Apply.
		currentState.HomeDir.Unknown = true
		currentState.Shell.Unknown = true
		currentState.UID.Unknown = true
		currentState.GID.Unknown = true

		return
	}

	// Update the current state with our user attrs.
	if user.HomeDir != nil {
		currentState.HomeDir.Set(*user.HomeDir)
	}
	if user.Shell != nil {
		currentState.Shell.Set(*user.Shell)
	}
	if user.UID != nil {
		currentState.UID.Set(*user.UID)
	}
	if user.GID != nil {
		currentState.GID.Set(*user.GID)
	}
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
//
// Importing a user doesn't make a lot of sense but we have to support the
// function regardless. As our only interface is a string ID, supporting this
// without provider level transport configuration would be absurdly difficult.
// Until then this will simply be a no-op. If/When we implement that behavior
// we could probably create use an identier that combines the source and
// destination to import a user.
func (f *user) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	newState := newUserStateV1()

	transportUtil.ImportResourceState(ctx, newState, req, res)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (f *user) PlanResourceChange(ctx context.Context, req resource.PlanResourceChangeRequest, res *resource.PlanResourceChangeResponse) {
	priorState := newUserStateV1()
	proposedState := newUserStateV1()
	res.PlannedState = proposedState

	transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, f, req, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	// Plan for all unknown computed attributes to be Unknown until after apply.
	if _, ok := priorState.ID.Get(); !ok {
		proposedState.ID.Unknown = true
	}

	if _, ok := proposedState.HomeDir.Get(); !ok {
		proposedState.HomeDir.Unknown = true
	}
	if _, ok := proposedState.Shell.Get(); !ok {
		proposedState.Shell.Unknown = true
	}
	if _, ok := proposedState.UID.Get(); !ok {
		proposedState.UID.Unknown = true
	}
	if _, ok := proposedState.GID.Get(); !ok {
		proposedState.GID.Unknown = true
	}
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a planned set of
// changes to the resource.
func (f *user) ApplyResourceChange(ctx context.Context, req resource.ApplyResourceChangeRequest, res *resource.ApplyResourceChangeResponse) {
	priorState := newUserStateV1()
	plannedState := newUserStateV1()
	res.NewState = plannedState

	transportUtil.ApplyUnmarshalState(ctx, priorState, plannedState, req, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	if req.IsDelete() || (!req.IsCreate() && !req.IsUpdate()) {
		// nothing to do on delete or when we're not creating or updating.
		return
	}

	transport := transportUtil.ApplyValidatePlannedAndBuildTransport(ctx, plannedState, f, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	plannedState.ID.Set("static")

	client, err := transport.Client(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Transport Error", err))
		return
	}
	defer client.Close()

	userSpec := plannedState.User()
	// Build our user create opts from our request. The only required option is "name".
	if userSpec == nil || userSpec.Name == nil || *userSpec.Name == "" {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Apply Error", fmt.Errorf("cannot create a user without a name")))
		return
	}

	opts := []remoteflight.UserOpt{remoteflight.WithUserName(*userSpec.Name)}
	if userSpec.HomeDir != nil {
		opts = append(opts, remoteflight.WithUserHomeDir(*userSpec.HomeDir))
	}
	if userSpec.Shell != nil {
		opts = append(opts, remoteflight.WithUserShell(*userSpec.Shell))
	}
	if userSpec.UID != nil {
		opts = append(opts, remoteflight.WithUserUID(*userSpec.UID))
	}
	if userSpec.GID != nil {
		opts = append(opts, remoteflight.WithUserGID(*userSpec.GID))
	}

	// Create or update our user.
	user, err := remoteflight.CreateOrUpdateUser(ctx, client, remoteflight.NewUser(opts...))
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnostic("Apply Error", err))
		return
	}

	// All of our attributes are either required or optional|computed. Make sure we've set them all.
	if user.Name != nil {
		plannedState.Name.Set(*user.Name)
	}
	if user.HomeDir != nil {
		plannedState.HomeDir.Set(*user.HomeDir)
	} else {
		plannedState.HomeDir.Set("")
	}
	if user.Shell != nil {
		plannedState.Shell.Set(*user.Shell)
	} else {
		plannedState.Shell.Set("")
	}
	if user.UID != nil {
		plannedState.UID.Set(*user.UID)
	} else {
		plannedState.UID.Set("")
	}
	if user.GID != nil {
		plannedState.GID.Set(*user.GID)
	} else {
		plannedState.GID.Set("")
	}
}

// User returns the state as a remoteflight User.
func (u *userStateV1) User() *remoteflight.User {
	if u == nil {
		return nil
	}

	user := &remoteflight.User{}
	if u.Name != nil {
		if n, ok := u.Name.Get(); ok {
			user.Name = &n
		}
	}
	if u.HomeDir != nil {
		if hd, ok := u.HomeDir.Get(); ok {
			user.HomeDir = &hd
		}
	}
	if u.Shell != nil {
		if sh, ok := u.Shell.Get(); ok {
			user.Shell = &sh
		}
	}
	if u.UID != nil {
		if uid, ok := u.UID.Get(); ok {
			user.UID = &uid
		}
	}
	if u.GID != nil {
		if gid, ok := u.GID.Get(); ok {
			user.GID = &gid
		}
	}

	return user
}

// Schema is the user states Terraform schema.
func (u *userStateV1) Schema() *tfprotov6.Schema {
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
					Name:     "name",
					Type:     tftypes.String,
					Required: true,
				},
				{
					Name:     "home_dir",
					Type:     tftypes.String,
					Computed: true,
					Optional: true,
				},
				{
					Name:     "shell",
					Type:     tftypes.String,
					Computed: true,
					Optional: true,
				},
				{
					Name:     "uid",
					Type:     tftypes.String,
					Computed: true,
					Optional: true,
				},
				{
					Name:     "gid",
					Type:     tftypes.String,
					Computed: true,
					Optional: true,
				},
				u.Transport.SchemaAttributeTransport(),
			},
		},
	}
}

// Validate validates the configuration. This will validate the source user
// exists and that the transport configuration is valid.
func (u *userStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// TODO

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (u *userStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id":       u.ID,
		"name":     u.Name,
		"home_dir": u.HomeDir,
		"shell":    u.Shell,
		"uid":      u.UID,
		"gid":      u.GID,
	})
	if err != nil {
		return err
	}

	if vals["transport"].IsKnown() {
		return u.Transport.FromTerraform5Value(vals["transport"])
	}

	return nil
}

// Terraform5Type is the user state tftypes.Type.
func (u *userStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":        u.ID.TFType(),
		"name":      u.Name.TFType(),
		"home_dir":  u.HomeDir.TFType(),
		"shell":     u.Shell.TFType(),
		"uid":       u.UID.TFType(),
		"gid":       u.GID.TFType(),
		"transport": u.Transport.Terraform5Type(),
	}}
}

// Terraform5Value is the user state tftypes.Value.
func (u *userStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(u.Terraform5Type(), map[string]tftypes.Value{
		"id":        u.ID.TFValue(),
		"name":      u.Name.TFValue(),
		"home_dir":  u.HomeDir.TFValue(),
		"shell":     u.Shell.TFValue(),
		"uid":       u.UID.TFValue(),
		"gid":       u.GID.TFValue(),
		"transport": u.Transport.Terraform5Value(),
	})
}

// EmbeddedTransport is a pointer to the state's embedded transport.
func (u *userStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return u.Transport
}
