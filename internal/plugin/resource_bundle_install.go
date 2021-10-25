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
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type bundleInstall struct {
	providerConfig *config
	mu             sync.Mutex
}

var _ resourcerouter.Resource = (*bundleInstall)(nil)

type bundleInstallStateV1 struct {
	ID          *tfString
	Path        *tfString
	Destination *tfString
	Release     *bundleInstallStateV1Release
	Artifactory *bundleInstallStateV1Artifactory
	Transport   *embeddedTransportV1
}

type bundleInstallStateV1Artifactory struct {
	Username *tfString
	Token    *tfString
	URL      *tfString
	SHA256   *tfString
}

type bundleInstallStateV1Release struct {
	Product *tfString
	Version *tfString
	Edition *tfString
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
		ID:          newTfString(),
		Path:        newTfString(),
		Destination: newTfString(),
		Artifactory: &bundleInstallStateV1Artifactory{
			Username: newTfString(),
			Token:    newTfString(),
			URL:      newTfString(),
			SHA256:   newTfString(),
		},
		Release: &bundleInstallStateV1Release{
			Product: newTfString(),
			Version: newTfString(),
			Edition: newTfString(),
		},
		Transport: newEmbeddedTransport(),
	}
}

func (r *bundleInstall) Name() string {
	return "enos_bundle_install"
}

func (r *bundleInstall) Schema() *tfprotov6.Schema {
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

// ValidateResourceConfig is the request Terraform sends when it wants to
// validate the resource's configuration.
func (r *bundleInstall) ValidateResourceConfig(ctx context.Context, req *tfprotov6.ValidateResourceConfigRequest) (*tfprotov6.ValidateResourceConfigResponse, error) {
	newState := newBundleInstallStateV1()

	return transportUtil.ValidateResourceConfig(ctx, newState, req)
}

// UpgradeResourceState is the request Terraform sends when it wants to
// upgrade the resource's state to a new version.
func (r *bundleInstall) UpgradeResourceState(ctx context.Context, req *tfprotov6.UpgradeResourceStateRequest) (*tfprotov6.UpgradeResourceStateResponse, error) {
	newState := newBundleInstallStateV1()

	return transportUtil.UpgradeResourceState(ctx, newState, req)
}

// ReadResource is the request Terraform sends when it wants to get the latest
// state for the resource.
func (r *bundleInstall) ReadResource(ctx context.Context, req *tfprotov6.ReadResourceRequest) (*tfprotov6.ReadResourceResponse, error) {
	newState := newBundleInstallStateV1()

	return transportUtil.ReadResource(ctx, newState, req)
}

// ImportResourceState is the request Terraform sends when it wants the provider
// to import one or more resources specified by an ID.
func (r *bundleInstall) ImportResourceState(ctx context.Context, req *tfprotov6.ImportResourceStateRequest) (*tfprotov6.ImportResourceStateResponse, error) {
	newState := newBundleInstallStateV1()

	return transportUtil.ImportResourceState(ctx, newState, req)
}

// PlanResourceChange is the request Terraform sends when it is generating a plan
// for the resource and wants the provider's input on what the planned state should be.
func (r *bundleInstall) PlanResourceChange(ctx context.Context, req *tfprotov6.PlanResourceChangeRequest) (*tfprotov6.PlanResourceChangeResponse, error) {
	priorState := newBundleInstallStateV1()
	proposedState := newBundleInstallStateV1()

	res, transport, err := transportUtil.PlanUnmarshalVerifyAndBuildTransport(ctx, priorState, proposedState, r, req)
	if err != nil {
		return res, err
	}

	if _, ok := proposedState.ID.Get(); !ok {
		proposedState.ID.Unknown = true
	}

	// Make sure that we set a default edition if we have a product
	if _, ok := proposedState.Release.Product.Get(); ok {
		if _, ok := proposedState.Release.Edition.Get(); !ok {
			proposedState.Release.Edition.Set("oss")
		}
	}

	err = transportUtil.PlanMarshalPlannedState(ctx, res, proposedState, transport)

	return res, err
}

// ApplyResourceChange is the request Terraform sends when it needs to apply a
// planned set of changes to the resource.
func (r *bundleInstall) ApplyResourceChange(ctx context.Context, req *tfprotov6.ApplyResourceChangeRequest) (*tfprotov6.ApplyResourceChangeResponse, error) {
	priorState := newBundleInstallStateV1()
	plannedState := newBundleInstallStateV1()

	res, err := transportUtil.ApplyUnmarshalState(ctx, priorState, plannedState, req)
	if err != nil {
		return res, err
	}

	// If we don't have destination, a required attribute, we must be deleting
	if _, ok := plannedState.Destination.Get(); !ok {
		// Delete the resource
		res.NewState, err = marshalDelete(plannedState)

		return res, err
	}

	transport, err := transportUtil.ApplyValidatePlannedAndBuildTransport(ctx, res, plannedState, r)
	if err != nil {
		return res, err
	}

	plannedState.ID.Set("static")

	ssh, err := transport.Client(ctx)
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, errToDiagnostic(err))
		return res, err
	}
	defer ssh.Close() //nolint: staticcheck

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

	if _, ok := s.Path.Get(); ok {
		return s.installFromPath(ctx, ssh)
	}

	_, pok := s.Release.Product.Get()
	_, vok := s.Release.Version.Get()
	if pok && vok {
		return s.installFromRelease(ctx, ssh)
	}

	return s.installFromArtifactory(ctx, ssh)
}

func (s *bundleInstallStateV1) rmPath(ctx context.Context, ssh it.Transport, path string) error {
	_, _, err := ssh.Run(ctx, command.New(fmt.Sprintf("rm -rf '%s'", path)))
	if err != nil {
		return wrapErrWithDiagnostics(err, "removing path", fmt.Sprintf("removing path %s", path))
	}

	return nil
}

func (s *bundleInstallStateV1) installFromPath(ctx context.Context, ssh it.Transport) error {
	path, ok := s.Path.Get()
	if !ok {
		return newErrWithDiagnostics("invalid configuration", "you must supply a path", "path")
	}

	src, err := tfile.Open(path)
	if err != nil {
		return wrapErrWithDiagnostics(err, "invalid configuration", "unable to open source bundle path", "path")
	}
	defer src.Close()

	bundlePath := fmt.Sprintf("/tmp/enos_bundle_install_%s.zip", random.ID())
	err = remoteflight.CopyFile(ctx, ssh, remoteflight.NewCopyFileRequest(
		remoteflight.WithCopyFileContent(src),
		remoteflight.WithCopyFileDestination(bundlePath),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "copy bundle", "unable to copy bundle to remote machine", "path")
	}

	dest, ok := s.Destination.Get()
	if !ok {
		return newErrWithDiagnostics("invalid configuration", "you must supply a destination", "destination")
	}

	_, err = remoteflight.Unzip(ctx, ssh, remoteflight.NewUnzipRequest(
		remoteflight.WithUnzipRequestSourcePath(bundlePath),
		remoteflight.WithUnzipRequestDestinationDir(dest),
		remoteflight.WithUnzipRequestUseSudo(true),
		remoteflight.WithUnzipRequestReplace(true),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "expand bundle", "unable to expand bundle zip file", "destination")
	}

	err = s.rmPath(ctx, ssh, bundlePath)
	if err != nil {
		return err
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

	prod, ok := s.Release.Product.Get()
	if !ok {
		return newErrWithDiagnostics("release", "you must supply a release product", "release", "product")
	}

	ver, ok := s.Release.Version.Get()
	if !ok {
		return newErrWithDiagnostics("release", "you must supply a release version", "release", "version")
	}

	ed, ok := s.Release.Edition.Get()
	if !ok {
		return newErrWithDiagnostics("release", "you must supply a release edition", "release", "edition")
	}

	release, err := releases.NewRelease(
		releases.WithReleaseProduct(prod),
		releases.WithReleaseVersion(ver),
		releases.WithReleaseEdition(ed),
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

	bundlePath := fmt.Sprintf("/tmp/enos_bundle_install_%s.zip", random.ID())
	_, err = remoteflight.Download(ctx, ssh, remoteflight.NewDownloadRequest(
		remoteflight.WithDownloadRequestDestination(bundlePath),
		remoteflight.WithDownloadRequestURL(release.BundleURL()),
		remoteflight.WithDownloadRequestSHA256(sha256),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "download bundle", "unable to download release bundle zip file")
	}

	dest, ok := s.Destination.Get()
	if !ok {
		return wrapErrWithDiagnostics(err, "release", "you must supply a destination", "destination")
	}

	_, err = remoteflight.Unzip(ctx, ssh, remoteflight.NewUnzipRequest(
		remoteflight.WithUnzipRequestSourcePath(bundlePath),
		remoteflight.WithUnzipRequestDestinationDir(dest),
		remoteflight.WithUnzipRequestUseSudo(true),
		remoteflight.WithUnzipRequestReplace(true),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "expand bundle", "unable to expand release bundle zip file", "destination")
	}

	err = s.rmPath(ctx, ssh, bundlePath)
	if err != nil {
		return err
	}

	return nil
}

func (s *bundleInstallStateV1) installFromArtifactory(ctx context.Context, ssh it.Transport) error {
	bundlePath := fmt.Sprintf("/tmp/enos_bundle_install_%s.zip", random.ID())

	url, ok := s.Artifactory.URL.Get()
	if !ok {
		return newErrWithDiagnostics("you must supply an artifactory url", "artifactory", "url")
	}

	username, ok := s.Artifactory.Username.Get()
	if !ok {
		return newErrWithDiagnostics("you must supply an artifactory username", "artifactory", "username")
	}

	token, ok := s.Artifactory.Token.Get()
	if !ok {
		return newErrWithDiagnostics("you must supply an artifactory token", "artifactory", "token")
	}

	sha, ok := s.Artifactory.SHA256.Get()
	if !ok {
		return newErrWithDiagnostics("you must supply an artifactory sha256", "artifactory", "sha256")
	}

	_, err := remoteflight.Download(ctx, ssh, remoteflight.NewDownloadRequest(
		remoteflight.WithDownloadRequestDestination(bundlePath),
		remoteflight.WithDownloadRequestURL(url),
		remoteflight.WithDownloadRequestAuthUser(username),
		remoteflight.WithDownloadRequestAuthPassword(token),
		remoteflight.WithDownloadRequestSHA256(sha),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "download bundle", "unable to download release bundle zip file")
	}

	dest, ok := s.Destination.Get()
	if !ok {
		return newErrWithDiagnostics("you must supply a destination", "destination")
	}

	_, err = remoteflight.Unzip(ctx, ssh, remoteflight.NewUnzipRequest(
		remoteflight.WithUnzipRequestSourcePath(bundlePath),
		remoteflight.WithUnzipRequestDestinationDir(dest),
		remoteflight.WithUnzipRequestUseSudo(true),
		remoteflight.WithUnzipRequestReplace(true),
	))
	if err != nil {
		return wrapErrWithDiagnostics(err, "expand bundle", "unable to expand release bundle zip file", "destination")
	}

	err = s.rmPath(ctx, ssh, bundlePath)
	if err != nil {
		return err
	}

	return nil
}

// Schema is the file states Terraform schema.
func (s *bundleInstallStateV1) Schema() *tfprotov6.Schema {
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
	if _, ok := s.Path.Get(); ok {
		sources++
	}

	if _, ok := s.Release.Product.Get(); ok {
		sources++
	}

	if _, ok := s.Artifactory.URL.Get(); ok {
		sources++
	}

	if sources == 0 {
		return newErrWithDiagnostics("invalid configuration", "no install source configured", "release", "product")
	} else if sources == 2 {
		return newErrWithDiagnostics("invalid configuration", "more than one install source configured", "release", "product")
	}

	// Make sure the path is valid if it is the install source
	if path, ok := s.Path.Get(); ok {
		p, err := filepath.Abs(path)
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
	if prod, ok := s.Release.Product.Get(); ok {
		if prod == "vault" {
			ed, ok := s.Release.Edition.Get()
			if !ok {
				return newErrWithDiagnostics("invalid configuration", "you must supply a vault edition", "release", "edition")
			}
			if !artifactory.SupportedVaultEdition(ed) {
				return newErrWithDiagnostics("invalid configuration", "unsupported vault edition", "release", "edition")
			}
		}
	}

	// Make sure that artifactory URL is a valid URL
	if u, ok := s.Artifactory.URL.Get(); ok {
		_, err := url.Parse(u)
		if err != nil {
			return wrapErrWithDiagnostics(err, "invalid configuration", "artifactory URL is invalid", "artifactory", "url")
		}
	}

	return nil
}

// FromTerraform5Value is a callback to unmarshal from the tftypes.Value with As().
func (s *bundleInstallStateV1) FromTerraform5Value(val tftypes.Value) error {
	vals, err := mapAttributesTo(val, map[string]interface{}{
		"id":          s.ID,
		"destination": s.Destination,
		"path":        s.Path,
	})
	if err != nil {
		return err
	}

	release, ok := vals["release"]
	if ok {
		if release.IsKnown() && !release.IsNull() {
			_, err = mapAttributesTo(release, map[string]interface{}{
				"product": s.Release.Product,
				"version": s.Release.Version,
				"edition": s.Release.Edition,
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
				"username": s.Artifactory.Username,
				"token":    s.Artifactory.Token,
				"url":      s.Artifactory.URL,
				"sha256":   s.Artifactory.SHA256,
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
		"id":          s.ID.TFType(),
		"destination": s.Destination.TFType(),
		"path":        s.Path.TFType(),
		"artifactory": s.ArtifactoryTerraform5Type(),
		"release":     s.ReleaseTerraform5Type(),
		"transport":   s.Transport.Terraform5Type(),
	}}
}

// Terraform5Type is the file state tftypes.Value.
func (s *bundleInstallStateV1) Terraform5Value() tftypes.Value {
	return tftypes.NewValue(s.Terraform5Type(), map[string]tftypes.Value{
		"id":          s.ID.TFValue(),
		"destination": s.Destination.TFValue(),
		"path":        s.Path.TFValue(),
		"artifactory": s.ArtifactoryTerraform5Value(),
		"release":     s.ReleaseTerraform5Value(),
		"transport":   s.Transport.Terraform5Value(),
	})
}

func (s *bundleInstallStateV1) ArtifactoryTerraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"username": s.Artifactory.Username.TFType(),
		"token":    s.Artifactory.Token.TFType(),
		"url":      s.Artifactory.URL.TFType(),
		"sha256":   s.Artifactory.SHA256.TFType(),
	}}
}

func (s *bundleInstallStateV1) ReleaseTerraform5Type() tftypes.Type {
	return tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"product": s.Release.Product.TFType(),
		"version": s.Release.Version.TFType(),
		"edition": s.Release.Edition.TFType(),
	}}
}

func (s *bundleInstallStateV1) ArtifactoryTerraform5Value() tftypes.Value {
	// As this is an optional value, return a nil object instead of nil values
	if tfStringsSetOrUnknown(s.Artifactory.Username, s.Artifactory.Token, s.Artifactory.URL) {
		return tftypes.NewValue(s.ArtifactoryTerraform5Type(), map[string]tftypes.Value{
			"username": s.Artifactory.Username.TFValue(),
			"token":    s.Artifactory.Token.TFValue(),
			"url":      s.Artifactory.URL.TFValue(),
			"sha256":   s.Artifactory.SHA256.TFValue(),
		})
	}

	return tftypes.NewValue(s.ArtifactoryTerraform5Type(), nil)
}

func (s *bundleInstallStateV1) ReleaseTerraform5Value() tftypes.Value {
	// As this is an optional value, return a nil object instead of nil values
	if tfStringsSetOrUnknown(s.Release.Product, s.Release.Version, s.Release.Edition) {
		return tftypes.NewValue(s.ReleaseTerraform5Type(), map[string]tftypes.Value{
			"product": s.Release.Product.TFValue(),
			"version": s.Release.Version.TFValue(),
			"edition": s.Release.Edition.TFValue(),
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
