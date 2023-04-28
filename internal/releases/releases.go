package releases

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Release represents a release from releases.hashicorp.com.
type Release struct {
	Product       string
	Version       string
	Edition       string
	Platform      string
	Arch          string
	GetSHA256Sums func(*Release) (string, error)
}

// ReleaseOpt is a function option for NewRelease.
type ReleaseOpt func(*Release) *Release

// NewRelease takes optional functional options and returns a new Release.
func NewRelease(opts ...ReleaseOpt) (*Release, error) {
	r := &Release{
		GetSHA256Sums: DefaultGetSHA256Sums,
	}

	for _, opt := range opts {
		r = opt(r)
	}

	return r, nil
}

// WithReleaseProduct sets the product.
func WithReleaseProduct(prod string) ReleaseOpt {
	return func(re *Release) *Release {
		re.Product = prod

		return re
	}
}

// WithReleaseVersion sets the version.
func WithReleaseVersion(ver string) ReleaseOpt {
	return func(re *Release) *Release {
		re.Version = ver

		return re
	}
}

// WithReleaseEdition sets the product.
func WithReleaseEdition(ed string) ReleaseOpt {
	return func(re *Release) *Release {
		re.Edition = ed

		return re
	}
}

// WithReleasePlatform sets the product.
func WithReleasePlatform(plat string) ReleaseOpt {
	return func(re *Release) *Release {
		re.Platform = plat

		return re
	}
}

// WithReleaseArch sets the product.
func WithReleaseArch(arch string) ReleaseOpt {
	return func(re *Release) *Release {
		re.Arch = arch

		return re
	}
}

// BundleURL returns the fully qualified URL to the release bundle archive.
func (r *Release) BundleURL() string {
	return fmt.Sprintf("%s%s", r.directoryURL(), r.bundleArtifactName())
}

// SHA256SUMSURL returns the fully qualified URL to the releases SHA256SUMS.
func (r *Release) SHA256SUMSURL() string {
	return fmt.Sprintf("%s%s_%s_SHA256SUMS", r.directoryURL(), r.Product, r.versionWithEdition())
}

func (r *Release) versionWithEdition() string {
	switch r.Edition {
	case "oss":
		return r.Version
	default:
		return fmt.Sprintf("%s+%s", r.Version, r.Edition)
	}
}

func (r *Release) directoryURL() string {
	return fmt.Sprintf("https://releases.hashicorp.com/%s/%s/", r.Product, r.versionWithEdition())
}

func (r *Release) bundleArtifactName() string {
	return fmt.Sprintf("%s_%s_%s_%s.zip", r.Product, r.versionWithEdition(), r.Platform, r.Arch)
}

// SHA256 parses the sums list for the release SHA256.
func (r *Release) SHA256() (string, error) {
	sums, err := r.GetSHA256Sums(r)
	if err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(strings.NewReader(sums))
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), r.bundleArtifactName()) {
			parts := strings.Split(scanner.Text(), " ")
			return parts[0], nil
		}
	}

	err = scanner.Err()
	if err != nil {
		return "", fmt.Errorf("unable to locate SHA256Sum for %s: %w", r.bundleArtifactName(), err)
	}

	return "", fmt.Errorf("unable to locate SHA256Sum for %s", r.bundleArtifactName())
}

// DefaultGetSHA256Sums attempts to download the SHA256Sums lists from the releases
// endpoint.
func DefaultGetSHA256Sums(rel *Release) (string, error) {
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, rel.SHA256SUMSURL(), nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("getting release SHA256SUMS: %s - %s", resp.Status, string(body))
	}

	return string(body), nil
}
