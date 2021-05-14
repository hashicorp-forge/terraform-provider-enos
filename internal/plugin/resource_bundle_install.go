package plugin

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/hashicorp/enos-provider/internal/artifactory"
	"github.com/hashicorp/enos-provider/internal/flightcontrol/remoteflight"
	"github.com/hashicorp/enos-provider/internal/random"
	"github.com/hashicorp/enos-provider/internal/releases"
	"github.com/hashicorp/enos-provider/internal/server/resourcerouter"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tftypes"
)

type bundleInstall struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resourcerouter.Resource = (*bundleInstall)(nil)

type bundleInstallStateV1 struct {
	ID          string
	Path        string
	Destination string
	Release     *bundleInstallStateV1Release
	Artifactory *bundleInstallStateV1Artifactory
	Transport   *embeddedTransportV1
}

type bundleInstallStateV1Artifactory struct {
	Username string
	Token    string
	URL      string
	SHA256   string
}

type bundleInstallStateV1Release struct {
	Product string
	Version string
	Edition string
}

var _ State = (*bundleInstallStateV1)(nil)

func newBundleInstall() *bundleInstall {
	return &bundleInstall{
		providerConfig: newProviderConfig(),
		mu:             sync.Mutex{},
	}
}

func newBundleInstallStateV1() *bundleInstallStateV1 {
	return &bundleInstallStateV1{
		Artifactory: &bundleInstallStateV1Artifactory{},
		Release:     &bundleInstallStateV1Release{},
		Transport:   newEmbeddedTransport(),
	}
}

func (r *bundleInstall) Name() string {
	return "enos_bundle_install"
}

func (r *bundleInstall) Schema() *tfprotov5.Schema {
	return newBundleInstallStateV1().Schema()
}

func (r *bundleInstall) SetProviderConfig(meta tftypes.Value) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.FromTerraform5Value(meta)
}

func (r *bundleInstall) GetProviderConfig() (*config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.providerConfig.Copy()
}

// ValidateResourceTypeConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (r *bundleInstall) ValidateResourceTypeConfig(ctx context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	newState := newBundleInstallStateV1()

	return transportUtil.ValidateResourceTypeConfig(ctx, newState, req)
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (r *bundleInstall) UpgradeResourceState(ctx context.Context, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
	newState := newBundleInstallStateV1()

	return transportUtil.UpgradeResourceState(ctx, newState, req)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *bundleInstall) ReadResource(ctx context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	newState := newBundleInstallStateV1()

	return transportUtil.ReadResource(ctx, newState, req)
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
func (r *bundleInstall) ImportResourceState(ctx context.Context, req *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
	newState := newBundleInstallStateV1()

	return transportUtil.ImportResourceState(ctx, newState, req)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *bundleInstall) PlanResourceChange(ctx context.Context, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, error) {
	priorState := newBundleInstallStateV1()
	proposedState := newBundleInstallStateV1()

	res, transport, err := transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, r, req)
	if err != nil {
		return res, err
	}

	// Handle setting the default edition
	if proposedState.Release.Product != "" {
		if proposedState.Release.Edition == "" {
			proposedState.Release.Edition = "oss"
		}
	}

	err = transportUtil.PlanMarshalPlannedState(ctx, res, proposedState, transport)

	return res, err
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *bundleInstall) ApplyResourceChange(ctx context.Context, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
	priorState := newBundleInstallStateV1()
	plannedState := newBundleInstallStateV1()

	res, err := transportUtil.ApplyUnmarshalState(ctx, priorState, plannedState, req)
	if err != nil {
		return res, err
	}

	// If we don't have destination, a required attribute, we must be deleting
	if plannedState.Destination == "" {
		// Delete the resource
		res.NewState, err = marshalDelete(plannedState)

		return res, err
	}

	transport, err := transportUtil.ApplyValidatePlannedAndBuildTransport(ctx, res, plannedState, r)
	if err != nil {
		return res, err
	}

	plannedState.ID = "static"

	ssh, err := transport.Client(ctx)
	defer ssh.Close() //nolint: staticcheck
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}

	if !priorState.equaltTo(plannedState) {
		err = plannedState.Install(ctx, ssh)
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
			return res, err
		}
	}

	err = transportUtil.ApplyMarshalNewState(ctx, res, plannedState, transport)

	return res, err
}

func (s *bundleInstallStateV1) Install(ctx context.Context, ssh it.Transport) error {
	// Install enos-flight-control because we need it to unzip bundles and download
	// bundles from the release endpoint or artifactory.
	_, err := remoteflight.Install(ctx, ssh, remoteflight.NewInstallRequest())
	if err != nil {
		return err
	}

	if s.Path != "" {
		return s.installFromPath(ctx, ssh)
	}

	if s.Release.Version != "" && s.Release.Product != "" {
		return s.installFromRelease(ctx, ssh)
	}

	return s.installFromArtifactory(ctx, ssh)
}

func (s *bundleInstallStateV1) mkTmpDir(ctx context.Context, ssh it.Transport) (string, error) {
	tmpDir := fmt.Sprintf("/tmp/enos_bundle_install_%s", random.ID())
	_, _, err := ssh.Run(ctx, command.New(fmt.Sprintf("mkdir -p %s", tmpDir)))
	if err != nil {
		return tmpDir, wrapErrWithDiagnostics(err, "create temporary directory", "unable to create temporary directory on destination host", "destination")
	}

	return tmpDir, nil
}

func (s *bundleInstallStateV1) rmDir(ctx context.Context, ssh it.Transport, dir string) error {
	_, _, err := ssh.Run(ctx, command.New(fmt.Sprintf("rm -rf '%s'", dir)))
	if err != nil {
		return wrapErrWithDiagnostics(err, "remove directory", "removing directory")
	}

	return nil
}

func (s *bundleInstallStateV1) installFromPath(ctx context.Context, ssh it.Transport) error {
	src, err := tfile.Open(s.Path)
	if err != nil {
		return wrapErrWithDiagnostics(err, "invalid configuration", "unable to open source bundle path", "path")
	}
	defer src.Close()

	tmpDir, err := s.mkTmpDir(ctx, ssh)
	if err != nil {
		return wrapErrWithDiagnostics(err, "transport", "creating temporary directory", "transport")
	}

	bundleFilePath := fmt.Sprintf("%s/bundle.zip", tmpDir)
	err = ssh.Copy(ctx, src, bundleFilePath)
	if err != nil {
		return wrapErrWithDiagnostics(err, "unable to copy", "unable copy bundle file", "path")
	}

	_, err = remoteflight.Unzip(ctx, ssh, remoteflight.NewUnzipRequest(
		remoteflight.WithUnzipRequestSourcePath(bundleFilePath),
		remoteflight.WithUnzipRequestDestinationDir(s.Destination),
		remoteflight.WithUnzipRequestUseSudo(true),
		remoteflight.WithUnzipRequestReplace(true),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "expand bundle", "unable to expand bundle zip file", "destination")
	}

	err = s.rmDir(ctx, ssh, tmpDir)
	if err != nil {
		return wrapErrWithDiagnostics(err, "transport", "removing temporary directory", "transport")
	}

	return nil
}

func (s *bundleInstallStateV1) installFromRelease(ctx context.Context, ssh it.Transport) error {
	platform, err := remoteflight.TargetPlatform(ctx, ssh)
	if err != nil {
		return wrapErrWithDiagnostics(err, "transport", "determining target host platform", "transport")
	}

	arch, err := remoteflight.TargetArchitecture(ctx, ssh)
	if err != nil {
		return wrapErrWithDiagnostics(err, "transport", "determining target host architecture", "transport")
	}

	release, err := releases.NewRelease(
		releases.WithReleaseProduct(s.Release.Product),
		releases.WithReleaseVersion(s.Release.Version),
		releases.WithReleaseEdition(s.Release.Edition),
		releases.WithReleasePlatform(platform),
		releases.WithReleaseArch(arch),
	)
	if err != nil {
		return wrapErrWithDiagnostics(err, "release", "determining release", "release")
	}

	sha256, err := release.SHA256()
	if err != nil {
		return wrapErrWithDiagnostics(err, "release", "determining release SHA", "release")
	}

	tmpDir, err := s.mkTmpDir(ctx, ssh)
	if err != nil {
		return wrapErrWithDiagnostics(err, "transport", "creating temporary directory", "transport")
	}

	bundleFilePath := fmt.Sprintf("%s/bundle.zip", tmpDir)

	_, err = remoteflight.Download(ctx, ssh, remoteflight.NewDownloadRequest(
		remoteflight.WithDownloadRequestDestination(bundleFilePath),
		remoteflight.WithDownloadRequestURL(release.BundleURL()),
		remoteflight.WithDownloadRequestSHA256(sha256),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "download bundle", "unable to download release bundle zip file")
	}

	_, err = remoteflight.Unzip(ctx, ssh, remoteflight.NewUnzipRequest(
		remoteflight.WithUnzipRequestSourcePath(bundleFilePath),
		remoteflight.WithUnzipRequestDestinationDir(s.Destination),
		remoteflight.WithUnzipRequestUseSudo(true),
		remoteflight.WithUnzipRequestReplace(true),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "expand bundle", "unable to expand release bundle zip file", "destination")
	}

	err = s.rmDir(ctx, ssh, tmpDir)
	if err != nil {
		return wrapErrWithDiagnostics(err, "transport", "removing temporary directory", "transport")
	}

	return nil
}

func (s *bundleInstallStateV1) installFromArtifactory(ctx context.Context, ssh it.Transport) error {
	tmpDir, err := s.mkTmpDir(ctx, ssh)
	if err != nil {
		return wrapErrWithDiagnostics(err, "transport", "creating temporary directory", "transport")
	}

	bundleFilePath := fmt.Sprintf("%s/bundle.zip", tmpDir)

	_, err = remoteflight.Download(ctx, ssh, remoteflight.NewDownloadRequest(
		remoteflight.WithDownloadRequestDestination(bundleFilePath),
		remoteflight.WithDownloadRequestURL(s.Artifactory.URL),
		remoteflight.WithDownloadRequestAuthUser(s.Artifactory.Username),
		remoteflight.WithDownloadRequestAuthPassword(s.Artifactory.Token),
		remoteflight.WithDownloadRequestSHA256(s.Artifactory.SHA256),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "download bundle", "unable to download release bundle zip file")
	}

	_, err = remoteflight.Unzip(ctx, ssh, remoteflight.NewUnzipRequest(
		remoteflight.WithUnzipRequestSourcePath(bundleFilePath),
		remoteflight.WithUnzipRequestDestinationDir(s.Destination),
		remoteflight.WithUnzipRequestUseSudo(true),
		remoteflight.WithUnzipRequestReplace(true),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "expand bundle", "unable to expand release bundle zip file", "destination")
	}

	err = s.rmDir(ctx, ssh, tmpDir)
	if err != nil {
		return wrapErrWithDiagnostics(err, "transport", "removing temporary directory", "transport")
	}

	return nil
}

// Schema is the file states Terraform schema.
func (s *bundleInstallStateV1) Schema() *tfprotov5.Schema {
	return &tfprotov5.Schema{
		Version: 1,
		Block: &tfprotov5.SchemaBlock{
			Attributes: []*tfprotov5.SchemaAttribute{
				{
					Name:     "id",
					Type:     tftypes.String,
					Computed: true,
				},
				{
					Name:     "destination",
					Type:     tftypes.String,
					Required: true,
				},
				{
					Name:     "path",
					Type:     tftypes.String,
					Optional: true,
				},
				{
					Name:      "artifactory",
					Type:      s.ArtifactoryTerraform5Type(),
					Sensitive: true, // mask the token
					Optional:  true,
				},
				{
					Name:     "release",
					Type:     s.ReleaseTerraform5Type(),
					Optional: true,
				},
				s.Transport.SchemaAttributeTransport(),
			},
		},
	}
}

// Validate validates the configuration. This will validate the source file
// exists and that the transport configuration is valid.
func (s *bundleInstallStateV1) Validate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Make sure that only one install source is configured
	sources := 0
	if s.Path != "" {
		sources++
	}

	if s.Release.Product != "" {
		sources++
	}

	if s.Artifactory.URL != "" {
		sources++
	}

	if sources == 0 {
		return newErrWithDiagnostics("invalid configuration", "no install source configured", "release", "product")
	} else if sources == 2 {
		return newErrWithDiagnostics("invalid configuration", "more than one install source configured", "release", "product")
	}

	// Make sure the path is valid if it is the install source
	if s.Path != "" {
		p, err := filepath.Abs(s.Path)
		if err != nil {
			return wrapErrWithDiagnostics(err, "invalid configuration", "unable to expand path", "path")
		}
		if strings.HasSuffix(p, string(os.PathSeparator)) {
			return newErrWithDiagnostics("invalid configuration", "path must not be a directory", "path")
		}
		_, err = os.Stat(filepath.Dir(p))
		if err != nil {
			return wrapErrWithDiagnostics(err, "invalid configuration", "path base directory does not exist", "path")
		}
	}

	// Make sure our product is a valid combination
	if s.Release.Product != "" {
		if s.Release.Product == "vault" {
			if !artifactory.SupportedVaultEdition(s.Release.Edition) {
				return newErrWithDiagnostics("invalid configuration", "unsupported vault edition", "release", "edition")
			}
		}
	}

	// Make sure that artifactory URL is a valid URL
	if s.Artifactory.URL != "" {
		_, err := url.Parse(s.Artifactory.URL)
		if err != nil {
			return newErrWithDiagnostics("invalid configuration", "artifactory URL is invalid", "artifactory", "url")
		}
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Value with As().
func (s *bundleInstallStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id":          &s.ID,
		"destination": &s.Destination,
		"path":        &s.Path,
	})
	if err != nil {
		return err
	}

	release, ok := vals["release"]
	if ok {
		if release.IsKnown() && !release.IsNull() {
			_, err = mapAttributesTo(release, map[string]interface{}{
				"product": &s.Release.Product,
				"version": &s.Release.Version,
				"edition": &s.Release.Edition,
			})
			if err != nil {
				return err
			}
		}
	}

	atf, ok := vals["artifactory"]
	if ok {
		if atf.IsKnown() && !atf.IsNull() {
			_, err = mapAttributesTo(atf, map[string]interface{}{
				"username": &s.Artifactory.Username,
				"token":    &s.Artifactory.Token,
				"url":      &s.Artifactory.URL,
				"sha256":   &s.Artifactory.SHA256,
			})
			if err != nil {
				return err
			}
		}
	}

	if !vals["transport"].IsKnown() {
		return nil
	}

	return s.Transport.FromTerraform5Value(vals["transport"])
}

// Terraform5Type is the file state tftypes.Type.
func (s *bundleInstallStateV1) Terraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"id":          tftypes.String,
		"destination": tftypes.String,
		"path":        tftypes.String,
		"artifactory": s.ArtifactoryTerraform5Type(),
		"release":     s.ReleaseTerraform5Type(),
		"transport":   s.Transport.Terraform5Type(),
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (s *bundleInstallStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":          tfMarshalStringValue(s.ID),
		"destination": tfMarshalStringValue(s.Destination),
		"path":        tfMarshalStringOptionalValue(s.Path),
		"artifactory": s.ArtifactoryTerraform5Value(),
		"release":     s.ReleaseTerraform5Value(),
		"transport":   s.Transport.Terraform5Value(),
	})
}

func (s *bundleInstallStateV1) ArtifactoryTerraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"username": tftypes.String,
		"token":    tftypes.String,
		"url":      tftypes.String,
		"sha256":   tftypes.String,
	}}
}

func (s *bundleInstallStateV1) ReleaseTerraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"product": tftypes.String,
		"version": tftypes.String,
		"edition": tftypes.String,
	}}
}

func (s *bundleInstallStateV1) ArtifactoryTerraform5Value() tftypes.Value {
	// As this is an optional value, return a nil object instead of nil values
	if tfStringsSetOrUnknown(s.Artifactory.Username, s.Artifactory.Token, s.Artifactory.URL) {
		return tftypes.NewValue(s.ArtifactoryTerraform5Type(), map[string]tftypes.Value{
			"username": tfMarshalStringValue(s.Artifactory.Username),
			"token":    tfMarshalStringValue(s.Artifactory.Token),
			"url":      tfMarshalStringValue(s.Artifactory.URL),
			"sha256":   tfMarshalStringValue(s.Artifactory.SHA256),
		})
	}

	return tftypes.NewValue(s.ArtifactoryTerraform5Type(), nil)
}

func (s *bundleInstallStateV1) ReleaseTerraform5Value() tftypes.Value {
	// As this is an optional value, return a nil object instead of nil values
	if tfStringsSetOrUnknown(s.Release.Product, s.Release.Version, s.Release.Edition) {
		return tftypes.NewValue(s.ReleaseTerraform5Type(), map[string]tftypes.Value{
			"product": tfMarshalStringValue(s.Release.Product),
			"version": tfMarshalStringValue(s.Release.Version),
			"edition": tfMarshalStringValue(s.Release.Edition),
		})
	}

	return tftypes.NewValue(s.ReleaseTerraform5Type(), nil)
}

func (s *bundleInstallStateV1) equaltTo(p *bundleInstallStateV1) bool {
	return reflect.DeepEqual(s, p)
}

// EmbeddedTransport returns a pointer the resources embedded transport.
func (s *bundleInstallStateV1) EmbeddedTransport() *embeddedTransportV1 {
	return s.Transport
}
