package remoteflight

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/hashicorp/enos-provider/internal/random"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
)

// Package install gets. These are the built-in package getters.
var (
	PackageInstallGetterCopy        = &PackageInstallGetter{"copy", packageInstallGetCopy}
	PackageInstallGetterReleases    = &PackageInstallGetter{"releases", packageInstallGetDownload}
	PackageInstallGetterArtifactory = &PackageInstallGetter{"artifactory", packageInstallGetDownload}
	PackageInstallGetterRepository  = &PackageInstallGetter{"repository", packageInstallGetRepository}
)

// Package install methods. These are the built-in pacakage installers.
var (
	PackageInstallInstallerZip = &PackageInstallInstaller{
		Type:    "zip",
		Install: packageInstallZipInstall,
		CompatibleGetters: []*PackageInstallGetter{
			PackageInstallGetterCopy,
			PackageInstallGetterReleases,
			PackageInstallGetterArtifactory,
		},
	}
	PackageInstallInstallerDEB = &PackageInstallInstaller{
		Type:    "deb",
		Install: packageInstallDEBInstall,
		CompatibleGetters: []*PackageInstallGetter{
			PackageInstallGetterCopy,
			PackageInstallGetterReleases,
			PackageInstallGetterArtifactory,
		},
	}
	PackageInstallInstallerRPM = &PackageInstallInstaller{
		Type:    "rpm",
		Install: packageInstallRPMInstall,
		CompatibleGetters: []*PackageInstallGetter{
			PackageInstallGetterCopy,
			PackageInstallGetterReleases,
			PackageInstallGetterArtifactory,
		},
	}
	PackageInstallInstallerYum = &PackageInstallInstaller{
		Type:    "yum",
		Install: packageInstallYumInstall,
		CompatibleGetters: []*PackageInstallGetter{
			PackageInstallGetterRepository,
		},
	}
	PackageInstallInstallerApt = &PackageInstallInstaller{
		Type:    "apt",
		Install: packageInstallAptInstall,
		CompatibleGetters: []*PackageInstallGetter{
			PackageInstallGetterRepository,
		},
	}
)

var (
	// ErrPackageInstallGetterUnknown means the package get has not been set.
	ErrPackageInstallGetterUnknown = fmt.Errorf("package install get is unknown")
	// ErrPackageInstallGetterUnsupported means the package get is unsupported not been set.
	ErrPackageInstallGetterUnsupported = fmt.Errorf("package install get is unsupported")
	// ErrPackageInstallInstallerUnknown means the package method has not been set.
	ErrPackageInstallInstallerUnknown = fmt.Errorf("package install method is unknown")
	// ErrPackageInstallInstallerUnsupported means the package method is unsupported.
	ErrPackageInstallInstallerUnsupported = fmt.Errorf("package install method is unsupported")
)

// PackageInstallInstaller is how a package is going to be installed.
type PackageInstallInstaller struct {
	Type              string
	CompatibleGetters []*PackageInstallGetter
	Install           func(ctx context.Context, ssh it.Transport, req *PackageInstallRequest) error
}

// Compatible determines if the install get artifact is compatible with
// the install method.
func (m *PackageInstallInstaller) Compatible(get *PackageInstallGetter) bool {
	for _, cs := range m.CompatibleGetters {
		if get.Type == cs.Type {
			return true
		}
	}

	return false
}

// PackageInstallGetter is where the package is coming from.
type PackageInstallGetter struct {
	Type string
	Get  func(ctx context.Context, ssh it.Transport, req *PackageInstallRequest) error
}

// PackageInstallRequest is a request to install a package on a target machine.
type PackageInstallRequest struct {
	Installer         *PackageInstallInstaller
	Getter            *PackageInstallGetter
	FlightControlPath string
	UnzipOpts         []UnzipOpt    // Unzip options if we're getting a zip bundle
	DownloadOpts      []DownloadOpt // Download options if we're downloading the artifact
	CopyPath          string        // Where to copy from
	TempArtifactPath  string        // Intermediate location of artifact
	TempDir           string        // Base directory of temporary directory
	DestionationPath  string        // Final destination of artifact
}

// PackageInstallResponse is the response of the script run.
type PackageInstallResponse struct{}

// PackageInstallRequestOpt is a functional option for running a script.
type PackageInstallRequestOpt func(*PackageInstallRequest) *PackageInstallRequest

// PackageInstallInstallerForFile attempts to determine a suitable package
// installation method given the file name.
func PackageInstallInstallerForFile(name string) *PackageInstallInstaller {
	switch filepath.Ext(filepath.Base(name)) {
	case ".zip":
		return PackageInstallInstallerZip
	case ".deb":
		return PackageInstallInstallerDEB
	case ".rpm":
		return PackageInstallInstallerRPM
	default:
		return nil
	}
}

// NewPackageInstallRequest takes functional options and returns a new script run req.
func NewPackageInstallRequest(opts ...PackageInstallRequestOpt) *PackageInstallRequest {
	ir := &PackageInstallRequest{
		UnzipOpts:         []UnzipOpt{},
		DownloadOpts:      []DownloadOpt{},
		TempDir:           "/tmp",
		FlightControlPath: DefaultFlightControlPath,
	}

	for _, opt := range opts {
		ir = opt(ir)
	}

	return ir
}

// WithPackageInstallInstaller sets the package installer.
func WithPackageInstallInstaller(method *PackageInstallInstaller) PackageInstallRequestOpt {
	return func(ir *PackageInstallRequest) *PackageInstallRequest {
		ir.Installer = method
		return ir
	}
}

// WithPackageInstallGetter sets the package install get.
func WithPackageInstallGetter(get *PackageInstallGetter) PackageInstallRequestOpt {
	return func(ir *PackageInstallRequest) *PackageInstallRequest {
		ir.Getter = get
		return ir
	}
}

// WithPackageInstallUnzipOpts sets the package unzip options.
func WithPackageInstallUnzipOpts(opts ...UnzipOpt) PackageInstallRequestOpt {
	return func(ir *PackageInstallRequest) *PackageInstallRequest {
		ir.UnzipOpts = opts
		return ir
	}
}

// WithPackageInstallDownloadOpts sets the package download options.
func WithPackageInstallDownloadOpts(opts ...DownloadOpt) PackageInstallRequestOpt {
	return func(ir *PackageInstallRequest) *PackageInstallRequest {
		ir.DownloadOpts = opts
		return ir
	}
}

// WithPackageInstallCopyPath sets get location of an artifact that
// is being copied from the local machine.
func WithPackageInstallCopyPath(path string) PackageInstallRequestOpt {
	return func(ir *PackageInstallRequest) *PackageInstallRequest {
		ir.CopyPath = path
		return ir
	}
}

// WithPackageInstallDestination sets final destination for binaries.
func WithPackageInstallDestination(path string) PackageInstallRequestOpt {
	return func(ir *PackageInstallRequest) *PackageInstallRequest {
		ir.DestionationPath = path
		return ir
	}
}

// WithPackageInstallTemporaryDirectory sets the temporary directory.
func WithPackageInstallTemporaryDirectory(dir string) PackageInstallRequestOpt {
	return func(ir *PackageInstallRequest) *PackageInstallRequest {
		ir.TempDir = dir
		return ir
	}
}

// PackageInstall copies the script to the remote host, executes it, and cleans it up.
func PackageInstall(ctx context.Context, ssh it.Transport, req *PackageInstallRequest) (*PackageInstallResponse, error) {
	res := &PackageInstallResponse{}

	if req.Getter == nil {
		return res, ErrPackageInstallGetterUnknown
	}

	if req.Installer == nil {
		return res, ErrPackageInstallInstallerUnknown
	}

	if !req.Installer.Compatible(req.Getter) {
		return res, fmt.Errorf("package installer %s is not compatible with package getter %s",
			req.Installer.Type,
			req.Getter.Type,
		)
	}

	err := req.Getter.Get(ctx, ssh, req)
	if err != nil {
		return res, err
	}

	return res, req.Installer.Install(ctx, ssh, req)
}

func packageInstallGetCopy(ctx context.Context, ssh it.Transport, req *PackageInstallRequest) error {
	if req.CopyPath == "" {
		return fmt.Errorf("you must supply a path to the get artifact you wish you copy")
	}

	src, err := tfile.Open(req.CopyPath)
	if err != nil {
		return fmt.Errorf("opening artifact to copy to remote host: %w", err)
	}
	defer src.Close()

	// TODO: Allow specifying a user so we can ensure that the temp directory
	// exists.
	req.TempArtifactPath = filepath.Join(
		req.TempDir,
		fmt.Sprintf("enos_install_get.%s.%s", random.ID(), filepath.Base(req.CopyPath)),
	)
	err = CopyFile(ctx, ssh, NewCopyFileRequest(
		WithCopyFileContent(src),
		WithCopyFileDestination(req.TempArtifactPath),
	))
	if err != nil {
		return fmt.Errorf("copying artifact to remote host: %w", err)
	}

	return nil
}

func packageInstallGetDownload(ctx context.Context, ssh it.Transport, req *PackageInstallRequest) error {
	_, err := InstallFlightControl(ctx, ssh, NewInstallFlightControlRequest(
		WithInstallFlightControlRequestPath(req.FlightControlPath),
	))
	if err != nil {
		return fmt.Errorf("installing flight-control binary to download package: %w", err)
	}

	// TODO: Allow specifying a user so we can ensure that the temp directory
	// exists.
	req.TempArtifactPath = filepath.Join(
		req.TempDir,
		fmt.Sprintf("enos_install_download.%s", random.ID()),
	)

	opts := []DownloadOpt{
		WithDownloadRequestDestination(req.TempArtifactPath),
		// since this request is run in a retry loop, we need to make sure to replace the file as it
		// could potentially exist from an earlier failed attempt to download
		WithDownloadRequestReplace(true),
	}
	opts = append(opts, req.DownloadOpts...)
	_, err = Download(ctx, ssh, NewDownloadRequest(opts...))
	if err != nil {
		return fmt.Errorf("downloading artifact to the remote host: %w", err)
	}

	return nil
}

func packageInstallGetRepository(ctx context.Context, ssh it.Transport, req *PackageInstallRequest) error {
	// TODO: right now this is just a shim because we assume the package repository
	// has the package. We could eventually check the package repo for the package.
	return nil
}

func packageInstallZipInstall(ctx context.Context, ssh it.Transport, req *PackageInstallRequest) error {
	_, err := InstallFlightControl(ctx, ssh, NewInstallFlightControlRequest(
		WithInstallFlightControlRequestPath(req.FlightControlPath),
	))
	if err != nil {
		return fmt.Errorf("installing flight-control binary to unzip bundle: %w", err)
	}

	opts := []UnzipOpt{
		WithUnzipRequestSourcePath(req.TempArtifactPath),
		WithUnzipRequestDestinationDir(req.DestionationPath),
		WithUnzipRequestUseSudo(true),
		WithUnzipRequestReplace(true),
	}
	opts = append(opts, req.UnzipOpts...)
	_, err = Unzip(ctx, ssh, NewUnzipRequest(opts...))
	if err != nil {
		return err
	}

	return DeleteFile(ctx, ssh, NewDeleteFileRequest(WithDeleteFilePath(req.TempArtifactPath)))
}

func packageInstallDEBInstall(ctx context.Context, client it.Transport, req *PackageInstallRequest) error {
	// If we have existing config files, we're assuming we want to keep them.
	// --force-confold defaults to using the existing files, instead of
	// interactively choosing which to use.
	cmd := fmt.Sprintf("sudo dpkg --force-confold --install %s", req.TempArtifactPath)
	stdout, stderr, err := client.Run(ctx, command.New(cmd))
	if err != nil {
		return WrapErrorWith(err, stdout, stderr, "installing debian package")
	}

	return DeleteFile(ctx, client, NewDeleteFileRequest(WithDeleteFilePath(req.TempArtifactPath)))
}

func packageInstallRPMInstall(ctx context.Context, client it.Transport, req *PackageInstallRequest) error {
	// NOTE: I don't like force here but it's the only way to make rpm
	// reinstall on update without lots of special logic. Eventually we could
	// get much more clever here to handle upgrade, reinstall, etc.
	cmd := fmt.Sprintf("sudo rpm -U --force %s", req.TempArtifactPath)
	stdout, stderr, err := client.Run(ctx, command.New(cmd))
	if err != nil {
		return WrapErrorWith(err, stdout, stderr, "installing rpm package")
	}

	return DeleteFile(ctx, client, NewDeleteFileRequest(WithDeleteFilePath(req.TempArtifactPath)))
}

func packageInstallYumInstall(ctx context.Context, ssh it.Transport, req *PackageInstallRequest) error {
	return ErrPackageInstallInstallerUnknown
}

func packageInstallAptInstall(ctx context.Context, ssh it.Transport, req *PackageInstallRequest) error {
	return ErrPackageInstallInstallerUnknown
}
