package plugin

import (
	"bytes"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceUser tests the user resource.
func TestAccResourceUser(t *testing.T) {
	t.Parallel()
	cfg := template.Must(template.New("enos_user").
		Funcs(transportRenderFunc).
		Parse(`resource "enos_user" "{{.ID.Value}}" {
		name = "{{.Name.Value}}"
		{{if .HomeDir.Value}}
		home_dir = "{{.HomeDir.Value}}"
		{{end}}
		{{if .Shell.Value}}
		shell = "{{.Shell.Value}}"
		{{end}}
		{{if .UID.Value}}
		uid = "{{.UID.Value}}"
		{{end}}
		{{if .GID.Value}}
		gid = "{{.GID.Value}}"
		{{end}}
		{{ renderTransport .Transport }}
	}`))

	cases := []testAccResourceTemplate{}

	user := newUserStateV1()
	user.ID.Set("foo")
	user.Name.Set("foo")
	user.HomeDir.Set("/Users/foo")
	user.Shell.Set("/bin/fish")
	user.UID.Set("999")
	user.GID.Set("111")
	ssh := newEmbeddedTransportSSH()
	ssh.User.Set("ubuntu")
	ssh.Host.Set("localhost")
	privateKey, err := readTestFile("../fixtures/ssh.pem")
	require.NoError(t, err)
	ssh.PrivateKey.Set(privateKey)
	require.NoError(t, user.Transport.SetTransportState(ssh))
	cases = append(cases, testAccResourceTemplate{
		"all fields are loaded correctly",
		user,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_user.foo", "id", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_user.foo", "stdout", regexp.MustCompile(`^$`)),
			resource.TestMatchResourceAttr("enos_user.foo", "stderr", regexp.MustCompile(`^$`)),
			resource.TestMatchResourceAttr("enos_user.foo", "name", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_user.foo", "home_dir", regexp.MustCompile(`^/Users/foo$`)),
			resource.TestMatchResourceAttr("enos_user.foo", "shell", regexp.MustCompile(`^/Users/foo$`)),
			resource.TestMatchResourceAttr("enos_user.foo", "gid", regexp.MustCompile(`^111$`)),
			resource.TestMatchResourceAttr("enos_user.foo", "uid", regexp.MustCompile(`^999$`)),
			resource.TestMatchResourceAttr("enos_user.foo", "transport.ssh.user", regexp.MustCompile(`^ubuntu$`)),
			resource.TestMatchResourceAttr("enos_user.foo", "transport.ssh.host", regexp.MustCompile(`^localhost$`)),
		),
		false,
	})

	//nolint:paralleltest// because our resource handles it
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
				ProtoV6ProviderFactories: testProviders(t),
				Steps:                    []resource.TestStep{step},
			})
		})
	}
}
