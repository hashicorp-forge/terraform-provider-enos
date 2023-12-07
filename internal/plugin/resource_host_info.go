package plugin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/enos-provider/internal/diags"
	"github.com/hashicorp/enos-provider/internal/remoteflight"
	"github.com/hashicorp/enos-provider/internal/retry"
	resource "github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	"github.com/hashicorp/enos-provider/internal/server/state"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type hostInfo struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resource.Resource = (*hostInfo)(nil)

type hostInfoStateV1 struct {
	ID *tfString

	Arch            *tfString
	Distro          *tfString
	DistroVersion   *tfString
	Hostname        *tfString
	Pid1            *tfString
	Platform        *tfString
	PlatformVersion *tfString

	Transport *embeddedTransportV1

	failureHandlers
}

var _ state.State = (*hostInfoStateV1)(nil)

func newHostInfo() *hostInfo {
	return &hostInfo{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newHostInfoStateV1() *hostInfoStateV1 {
	transport := newEmbeddedTransport()
	fh := failureHandlers{TransportDebugFailureHandler(transport)}

	return &hostInfoStateV1{
		ID: newTfString(),

		Arch:            newTfString(),
		Distro:          newTfString(),
		DistroVersion:   newTfString(),
		Hostname:        newTfString(),
		Pid1:            newTfString(),
		Platform:        newTfString(),
		PlatformVersion: newTfString(),

		// TODO: Add support for determining the default package manager if there is one?

		Transport:       transport,
		failureHandlers: fh,
	}
}

func (f *hostInfo) Name() string {
	return "enos_host_info"
}

func (f *hostInfo) Schema() *tfprotov6.Schema {
	return newHostInfoStateV1().Schema()
}

func (f *hostInfo) SetProviderConfig(providerConfig tftypes.Value) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.providerConfig.FromTerraform5Value(providerConfig)
}

func (f *hostInfo) GetProviderConfig() (*config, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.providerConfig.Copy()
}

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (f *hostInfo) ValidateResourceConfig(ctx context.Context, req tfprotov6.ValidateResourceConfigRequest, res *tfprotov6.ValidateResourceConfigResponse) {
	newState := newHostInfoStateV1()

	transportUtil.ValidateResourceConfig(ctx, newState, req, res)
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (f *hostInfo) UpgradeResourceState(ctx context.Context, req tfprotov6.UpgradeResourceStateRequest, res *tfprotov6.UpgradeResourceStateResponse) {
	newState := newHostInfoStateV1()

	transportUtil.UpgradeResourceState(ctx, newState, req, res)
}

// ReadResource is the request Terraform sends when it wants to get the latest state for the resource.
// We'll exit gracefully if we're unable to read the resource since it's possible that it does not
// yet exist.
func (f *hostInfo) ReadResource(ctx context.Context, req tfprotov6.ReadResourceRequest, res *tfprotov6.ReadResourceResponse) {
	select {
	case <-ctx.Done():
		res.Diagnostics = append(res.Diagnostics, ctxToDiagnostic(ctx))
		return
	default:
	}

	currentState := newHostInfoStateV1()

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

	client, err := transport.Client(ctx)
	if err != nil {
		return
	}
	defer client.Close()

	// We actually have a valid transport and hostInfo name. Try and find the hostInfo.
	hostInfo, _ := remoteflight.TargetHostInfo(ctx, client, remoteflight.NewTargetRequest(
		remoteflight.WithTargetRequestRetryOpts(
			retry.WithIntervalFunc(retry.IntervalExponential(2*time.Second)),
			retry.WithMaxRetries(3),
		),
	))

	if hostInfo == nil {
		// We couldn't find our HostInfo. Set all of our current state attrs to Unknown since we can't read
		// it and need to Apply.
		currentState.Arch.Unknown = true
		currentState.Distro.Unknown = true
		currentState.DistroVersion.Unknown = true
		currentState.Hostname.Unknown = true
		currentState.Pid1.Unknown = true
		currentState.Platform.Unknown = true
		currentState.PlatformVersion.Unknown = true

		return
	}

	// Update the current state with our hostInfo attrs.
	currentState.SetInfo(hostInfo)
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
//
// Importing a hostInfo doesn't make a lot of sense but we have to support the
// function regardless. As our only interface is a string ID, supporting this
// without provider level transport configuration would be absurdly difficult.
// Until then this will simply be a no-op. If/When we implement that behavior
// we could probably create use an identier that combines the source and
// destination to import a hostInfo.
func (f *hostInfo) ImportResourceState(ctx context.Context, req tfprotov6.ImportResourceStateRequest, res *tfprotov6.ImportResourceStateResponse) {
	newState := newHostInfoStateV1()

	transportUtil.ImportResourceState(ctx, newState, req, res)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (f *hostInfo) PlanResourceChange(ctx context.Context, req resource.PlanResourceChangeRequest, res *resource.PlanResourceChangeResponse) {
	priorState := newHostInfoStateV1()
	proposedState := newHostInfoStateV1()
	res.PlannedState = proposedState

	transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, f, req, res)
	if diags.HasErrors(res.Diagnostics) {
		return
	}

	// Plan for all unknown computed attributes to be Unknown until after apply.
	if _, ok := priorState.ID.Get(); !ok {
		proposedState.ID.Unknown = true
	}
	if _, ok := proposedState.Arch.Get(); !ok {
		proposedState.Arch.Unknown = true
	}
	if _, ok := proposedState.Distro.Get(); !ok {
		proposedState.Distro.Unknown = true
	}
	if _, ok := proposedState.DistroVersion.Get(); !ok {
		proposedState.DistroVersion.Unknown = true
	}
	if _, ok := proposedState.Hostname.Get(); !ok {
		proposedState.Hostname.Unknown = true
	}
	if _, ok := proposedState.Pid1.Get(); !ok {
		proposedState.Pid1.Unknown = true
	}
	if _, ok := proposedState.Platform.Get(); !ok {
		proposedState.Platform.Unknown = true
	}
	if _, ok := proposedState.PlatformVersion.Get(); !ok {
		proposedState.PlatformVersion.Unknown = true
	}
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a planned set of
// changes to the resource.
func (f *hostInfo) ApplyResourceChange(ctx context.Context, req resource.ApplyResourceChangeRequest, res *resource.ApplyResourceChangeResponse) {
	priorState := newHostInfoStateV1()
	plannedState := newHostInfoStateV1()
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

	hostInfo, err := remoteflight.TargetHostInfo(ctx, client, remoteflight.NewTargetRequest(
		remoteflight.WithTargetRequestRetryOpts(
			retry.WithIntervalFunc(retry.IntervalExponential(2*time.Second)),
			retry.WithMaxRetries(3),
		),
	))
	// Not all platforms can be expected to have all host info. If we get an error return a warning
	// diagnostic for now.
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, diags.ErrToDiagnosticWarn(
			"getting target host info",
			AttributePathError(
				fmt.Errorf("failed to determine host information, due to: %w", err),
				"transport",
			),
		))
	}

	// Update the current state with our hostInfo attrs.
	plannedState.SetInfo(hostInfo)
}

func (s *hostInfoStateV1) SetInfo(i *remoteflight.HostInfo) {
	if i == nil {
		return
	}

	// Update the current state with our hostInfo attrs.
	if i.Arch != nil {
		s.Arch.Set(*i.Arch)
	} else {
		s.Arch.Set("")
	}
	if i.Distro != nil {
		s.Distro.Set(*i.Distro)
	} else {
		s.Distro.Set("")
	}
	if i.DistroVersion != nil {
		s.DistroVersion.Set(*i.DistroVersion)
	} else {
		s.DistroVersion.Set("")
	}
	if i.Hostname != nil {
		s.Hostname.Set(*i.Hostname)
	} else {
		s.Hostname.Set("")
	}
	if i.Pid1 != nil {
		s.Pid1.Set(*i.Pid1)
	} else {
		s.Pid1.Set("")
	}
	if i.Platform != nil {
		s.Platform.Set(*i.Platform)
	} else {
		s.Platform.Set("")
	}
	if i.PlatformVersion != nil {
		s.PlatformVersion.Set(*i.PlatformVersion)
	} else {
		s.PlatformVersion.Set("")
	}
}

// Schema is the hostInfo states Terraform schema.
func (s *hostInfoStateV1) Schema() *tfprotov6.Schema {
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
					Name:     "arch",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "distro",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "distro_version",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "hostname",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "pid1",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "platform",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "platform_version",
					Type:     tftypes.String,
					Computed: true,
				},
				s.Transport.SchemaAttributeTransport(),
			},
		},
	}
}

// Validate validates the configuration. This will validate the source hostInfo
// exists and that the transport configuration is valid.
func (s *hostInfoStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Vault with As().
func (s *hostInfoStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id":               s.ID,
		"arch":             s.Arch,
		"distro":           s.Distro,
		"distro_version":   s.DistroVersion,
		"hostname":         s.Hostname,
		"pid1":             s.Pid1,
		"platform":         s.Platform,
		"platform_version": s.PlatformVersion,
	})
	if err != nil {
		return err
	}

	if vals["transport"].IsKnown() {
		return s.Transport.FromTerraform5Value(vals["transport"])
	}

	return nil
}

// Terraform5Type is the hostInfo state tftypes.Type.
func (s *hostInfoStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":               s.ID.TFType(),
		"arch":             s.Arch.TFType(),
		"distro":           s.Distro.TFType(),
		"distro_version":   s.DistroVersion.TFType(),
		"hostname":         s.Hostname.TFType(),
		"pid1":             s.Pid1.TFType(),
		"platform":         s.Platform.TFType(),
		"platform_version": s.PlatformVersion.TFType(),
		"transport":        s.Transport.Terraform5Type(),
	}}
}

// Terraform5Value is the hostInfo state tftypes.Value.
func (s *hostInfoStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":               s.ID.TFValue(),
		"arch":             s.Arch.TFValue(),
		"distro":           s.Distro.TFValue(),
		"distro_version":   s.DistroVersion.TFValue(),
		"hostname":         s.Hostname.TFValue(),
		"pid1":             s.Pid1.TFValue(),
		"platform":         s.Platform.TFValue(),
		"platform_version": s.PlatformVersion.TFValue(),
		"transport":        s.Transport.Terraform5Value(),
	})
}

// EmbeddedTransport is a pointer to the state's embedded transport.
func (s *hostInfoStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}
