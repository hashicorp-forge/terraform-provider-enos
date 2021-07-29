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
	cfg := template.Must(template.New("enos_bundle_install").Parse(`resource "enos_bundle_install" "{{.ID.Value}}" {
		destination = "{{ .Destination.Value }}"

		{{ if .Path.Value -}}
		path = "{{ .Path.Value }}"
		{{ end -}}

		{{ if .Release.Product.Value -}}
		release = {
			product  = "{{ .Release.Product.Value }}"
			version  = "{{ .Release.Version.Value }}"
			edition  = "{{ .Release.Edition.Value }}"
		}
		{{ end -}}

		{{ if .Artifactory.URL.Value -}}
		artifactory = {
			username = "{{ .Artifactory.Username.Value }}"
			token    = "{{ .Artifactory.Token.Value }}"
			url      = "{{ .Artifactory.URL.Value }}"
			sha256   = "{{ .Artifactory.SHA256.Value }}"
		}
		{{ end -}}

		transport = {
			ssh = {
				{{if .Transport.SSH.User.Value}}
				user = "{{.Transport.SSH.User.Value}}"
				{{end}}

				{{if .Transport.SSH.Host.Value}}
				host = "{{.Transport.SSH.Host.Value}}"
				{{end}}

				{{if .Transport.SSH.PrivateKey.Value}}
				private_key = <<EOF
   {{.Transport.SSH.PrivateKey.Value}}
   EOF
				{{end}}

				{{if .Transport.SSH.PrivateKeyPath.Value}}
				private_key_path = "{{.Transport.SSH.PrivateKeyPath.Value}}"
				{{end}}

				{{if .Transport.SSH.Passphrase.Value}}
				passphrase = "{{.Transport.SSH.Passphrase.Value}}"
				{{end}}

				{{if .Transport.SSH.PassphrasePath}}
				passphrase_path = "{{.Transport.SSH.PassphrasePath.Value}}"
				{{end}}
			}
		}
}`))

	cases := []testAccResourceTemplate{}
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)

	installBundlePath := newBundleInstallStateV1()
	installBundlePath.ID.Set("path")
	installBundlePath.Destination.Set("/usr/local/bin/vault")
	installBundlePath.Path.Set("/some/local/path")
	installBundlePath.Transport.SSH.User.Set("ubuntu")
	installBundlePath.Transport.SSH.Host.Set("localhost")
	installBundlePath.Transport.SSH.PrivateKey.Set(privateKey)
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
	installBundleRelease.ID.Set("release")
	installBundleRelease.Destination.Set("/usr/local/bin/vault")
	installBundleRelease.Release.Product.Set("vault")
	installBundleRelease.Release.Version.Set("1.7.0")
	installBundleRelease.Release.Edition.Set("ent")
	installBundleRelease.Transport.SSH.User.Set("ubuntu")
	installBundleRelease.Transport.SSH.Host.Set("localhost")
	installBundleRelease.Transport.SSH.PrivateKey.Set(privateKey)
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
	installBundleArtifactory.ID.Set("art")
	installBundleArtifactory.Destination.Set("/opt/vault/bin")
	installBundleArtifactory.Artifactory.Token.Set("1234abcd")
	installBundleArtifactory.Artifactory.Username.Set("some@user.com")
	installBundleArtifactory.Artifactory.URL.Set("https://artifactory.com")
	installBundleArtifactory.Artifactory.SHA256.Set("abcd1234")
	installBundleArtifactory.Transport.SSH.User.Set("ubuntu")
	installBundleArtifactory.Transport.SSH.Host.Set("localhost")
	installBundleArtifactory.Transport.SSH.PrivateKey.Set(privateKey)
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

	// To do a real test, set the environment variables when running `make test-acc`
	enosVars, ok := ensureEnosTransportEnvVars(t)
	if ok {
		bundleInstallRealPathInstall := newBundleInstallStateV1()
		bundleInstallRealPathInstall.ID.Set("realpath")
		bundleInstallRealPathInstall.Path.Set("../fixtures/bundle.zip")
		bundleInstallRealPathInstall.Destination.Set("/opt/vault/bin")
		bundleInstallRealPathInstall.Transport.SSH.Host.Set(enosVars["host"])
		cases = append(cases, testAccResourceTemplate{
			"real_path",
			bundleInstallRealPathInstall,
			resource.ComposeTestCheckFunc(),
			true,
		})

		bundleInstallReleaseInstall := newBundleInstallStateV1()
		bundleInstallReleaseInstall.ID.Set("realrelease")
		bundleInstallReleaseInstall.Destination.Set("/opt/boundary/bin")
		bundleInstallReleaseInstall.Release.Edition.Set("oss")
		bundleInstallReleaseInstall.Release.Product.Set("boundary") // use boundary 0.1.0 cause it's not a big bundle
		bundleInstallReleaseInstall.Release.Version.Set("0.1.0")
		bundleInstallReleaseInstall.Transport.SSH.Host.Set(enosVars["host"])
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
			bundleInstallArtifactoryInstall.ID.Set("realart")
			bundleInstallArtifactoryInstall.Destination.Set("/opt/vault/bin")
			bundleInstallArtifactoryInstall.Artifactory.Username.Set(artUser)
			bundleInstallArtifactoryInstall.Artifactory.Token.Set(artToken)
			bundleInstallArtifactoryInstall.Artifactory.URL.Set("https://artifactory.hashicorp.engineering/artifactory/hashicorp-packagespec-buildcache-local/cache-v1/vault-enterprise/7fb88d4d3d0a36ffc78a522d870492e5791bae1b0640232ce4c6d69cc22cf520/store/f45845666b4e552bfc8ca775834a3ef6fc097fe0-1a2809da73e5896b6f766b395ff6e1804f876c45.zip")
			bundleInstallArtifactoryInstall.Artifactory.SHA256.Set("d01a82111133908167a5a140604ab3ec8fd18601758376a5f8e9dd54c7703373")
			bundleInstallArtifactoryInstall.Transport.SSH.Host.Set(enosVars["host"])
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
