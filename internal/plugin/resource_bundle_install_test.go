package plugin

import (
	"bytes"
	"os"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceBundleInstall tests the bundle_install resource
func TestAccResourceBundleInstall(t *testing.T) {
	cfg := template.Must(template.New("enos_bundle_install").Parse(`resource "enos_bundle_install" "{{.ID}}" {
		destination = "{{ .Destination }}"

		{{ if .Path -}}
		path = "{{ .Path }}"
		{{ end -}}

		{{ if .Release.Product -}}
		release = {
			product  = "{{ .Release.Product }}"
			version  = "{{ .Release.Version }}"
			edition  = "{{ .Release.Edition }}"
		}
		{{ end -}}

		{{ if .Artifactory.URL -}}
		artifactory = {
			username = "{{ .Artifactory.Username }}"
			token    = "{{ .Artifactory.Token }}"
			url      = "{{ .Artifactory.URL }}"
			sha256   = "{{ .Artifactory.SHA256 }}"
		}
		{{ end -}}

		transport = {
			ssh = {
				{{if .Transport.SSH.User}}
				user = "{{.Transport.SSH.User}}"
				{{end}}

				{{if .Transport.SSH.Host}}
				host = "{{.Transport.SSH.Host}}"
				{{end}}

				{{if .Transport.SSH.PrivateKey}}
				private_key = <<EOF
   {{.Transport.SSH.PrivateKey}}
   EOF
				{{end}}

				{{if .Transport.SSH.PrivateKeyPath}}
				private_key_path = "{{.Transport.SSH.PrivateKeyPath}}"
				{{end}}

				{{if .Transport.SSH.Passphrase}}
				passphrase = "{{.Transport.SSH.Passphrase}}"
				{{end}}

				{{if .Transport.SSH.PassphrasePath}}
				passphrase_path = "{{.Transport.SSH.PassphrasePath}}"
				{{end}}
			}
		}
}`))

	cases := []testAccResourceTemplate{}
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)

	installBundlePath := newBundleInstallStateV1()
	installBundlePath.ID = "path"
	installBundlePath.Destination = "/usr/local/bin/vault"
	installBundlePath.Path = "/some/local/path"
	installBundlePath.Transport.SSH.User = "ubuntu"
	installBundlePath.Transport.SSH.Host = "localhost"
	installBundlePath.Transport.SSH.PrivateKey = privateKey
	cases = append(cases, testAccResourceTemplate{
		"path",
		installBundlePath,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_bundle_install.path", "id", regexp.MustCompile(`^path$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.path", "destination", regexp.MustCompile(`^/usr/local/bin/vault$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.path", "path", regexp.MustCompile(`^/some/local/path$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.path", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.path", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
		),
		false,
	})

	installBundleRelease := newBundleInstallStateV1()
	installBundleRelease.ID = "release"
	installBundleRelease.Destination = "/usr/local/bin/vault"
	installBundleRelease.Release.Product = "vault"
	installBundleRelease.Release.Version = "1.7.0"
	installBundleRelease.Release.Edition = "ent"
	installBundleRelease.Transport.SSH.User = "ubuntu"
	installBundleRelease.Transport.SSH.Host = "localhost"
	installBundleRelease.Transport.SSH.PrivateKey = privateKey
	cases = append(cases, testAccResourceTemplate{
		"release",
		installBundleRelease,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_bundle_install.release", "id", regexp.MustCompile(`^path$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.release", "destination", regexp.MustCompile(`^/usr/local/bin/vault$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.release", "release.product", regexp.MustCompile(`^vault$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.release", "release.version", regexp.MustCompile(`^1.7.0$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.release", "release.edition", regexp.MustCompile(`^ent$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.release", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.release", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
		),
		false,
	})

	installBundleArtifactory := newBundleInstallStateV1()
	installBundleArtifactory.ID = "art"
	installBundleArtifactory.Destination = "/opt/vault/bin"
	installBundleArtifactory.Artifactory.Token = "1234abcd"
	installBundleArtifactory.Artifactory.Username = "some@user.com"
	installBundleArtifactory.Artifactory.URL = "https://artifactory.com"
	installBundleArtifactory.Artifactory.SHA256 = "abcd1234"
	installBundleArtifactory.Transport.SSH.User = "ubuntu"
	installBundleArtifactory.Transport.SSH.Host = "localhost"
	installBundleArtifactory.Transport.SSH.PrivateKey = privateKey
	cases = append(cases, testAccResourceTemplate{
		"artifactory",
		installBundleArtifactory,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_bundle_install.art", "id", regexp.MustCompile(`^path$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.art", "destination", regexp.MustCompile(`^/usr/local/bin/vault$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.art", "artifactory.username", regexp.MustCompile(`^some@user.com$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.art", "artifactory.token", regexp.MustCompile(`^1234abcd$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.art", "artifactory.url", regexp.MustCompile(`^https://artifactory.com$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.art", "artifactory.sha256", regexp.MustCompile(`^abcd1234$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.art", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_bundle_install.art", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
		),
		false,
	})

	// To do a real test, set the environment variables when running `make testacc`
	enosVars, ok := ensureEnosTransportEnvVars(t)
	if ok {
		bundleInstallRealPathInstall := newBundleInstallStateV1()
		bundleInstallRealPathInstall.ID = "real"
		bundleInstallRealPathInstall.Path = "../fixtures/bundle.zip"
		bundleInstallRealPathInstall.Destination = "/opt/vault/bin"
		bundleInstallRealPathInstall.Transport.SSH.Host = enosVars["host"]
		cases = append(cases, testAccResourceTemplate{
			"real_path",
			bundleInstallRealPathInstall,
			resource.ComposeTestCheckFunc(),
			true,
		})

		bundleInstallReleaseInstall := newBundleInstallStateV1()
		bundleInstallReleaseInstall.ID = "real"
		bundleInstallReleaseInstall.Destination = "/opt/boundary/bin"
		bundleInstallReleaseInstall.Release.Edition = "oss"
		bundleInstallReleaseInstall.Release.Product = "boundary" // use boundary 0.1.0 cause it's not a big bundle
		bundleInstallReleaseInstall.Release.Version = "0.1.0"
		bundleInstallReleaseInstall.Transport.SSH.Host = enosVars["host"]
		cases = append(cases, testAccResourceTemplate{
			"real_release",
			bundleInstallReleaseInstall,
			resource.ComposeTestCheckFunc(),
			true,
		})

		artUser, okuser := os.LookupEnv("ARTIFACTORY_USER")
		artToken, oktoken := os.LookupEnv("ARTIFACTORY_TOKEN")
		if !(oktoken && okuser) {
			t.Log(`skipping data bundle install from artifactory test because TF_ACC, ARTIFACTORY_TOKEN, ARTIFACTORY_USER aren't set`)
			t.Skip()
		} else {
			bundleInstallArtifactoryInstall := newBundleInstallStateV1()
			bundleInstallArtifactoryInstall.ID = "real"
			bundleInstallArtifactoryInstall.Destination = "/opt/vault/bin"
			bundleInstallArtifactoryInstall.Artifactory.Username = artUser
			bundleInstallArtifactoryInstall.Artifactory.Token = artToken
			bundleInstallArtifactoryInstall.Artifactory.URL = "https://artifactory.hashicorp.engineering/artifactory/hashicorp-packagespec-buildcache-local/cache-v1/vault-enterprise/7fb88d4d3d0a36ffc78a522d870492e5791bae1b0640232ce4c6d69cc22cf520/store/f45845666b4e552bfc8ca775834a3ef6fc097fe0-1a2809da73e5896b6f766b395ff6e1804f876c45.zip"
			bundleInstallArtifactoryInstall.Artifactory.SHA256 = "d01a82111133908167a5a140604ab3ec8fd18601758376a5f8e9dd54c7703373"
			bundleInstallArtifactoryInstall.Transport.SSH.Host = enosVars["host"]
			cases = append(cases, testAccResourceTemplate{
				"real_art",
				bundleInstallArtifactoryInstall,
				resource.ComposeTestCheckFunc(),
				true,
			})
		}
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			buf := bytes.Buffer{}
			err := cfg.Execute(&buf, test.state)
			if err != nil {
				t.Fatalf("error executing test template: %s", err.Error())
			}

			step := resource.TestStep{
				Config: buf.String(),
				Check:  test.check,
			}

			if !test.apply {
				step.PlanOnly = true
				step.ExpectNonEmptyPlan = true
			}

			resource.ParallelTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProviders,
				Steps:                    []resource.TestStep{step},
			})
		})
	}
}
