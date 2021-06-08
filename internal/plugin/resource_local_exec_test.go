package plugin

import (
	"bytes"
	"regexp"
	"testing"
	"text/template"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceLocalExec tests the local_exec resource
func TestAccResourceLocalExec(t *testing.T) {
	cfg := template.Must(template.New("enos_local_exec").Parse(`resource "enos_local_exec" "{{.ID}}" {
        {{if .Content}}
        content = <<EOF
{{.Content}}
EOF
        {{end}}
        {{if .Inline}}
        inline = [
        {{range .Inline}}
            "{{.}}",
        {{end}}
        ]
        {{end}}
        {{if .Scripts}}
        scripts = [
        {{range .Scripts}}
            "{{.}}",
        {{end}}
        ]
        {{end}}
        {{if .Env}}
        environment = {
        {{range $name, $val := .Env}}
            "{{$name}}": "{{$val}}",
        {{end}}
        }
        {{end}}
    }`))
	cases := []testAccResourceTemplate{}
	localExec := newLocalExecStateV1()
	localExec.ID = "foo"
	localExec.Env = map[string]string{"FOO": "BAR"}
	localExec.Scripts = []string{"../fixtures/src.txt"}
	localExec.Inline = []string{"touch /tmp/foo"}
	localExec.Content = "some content"
	cases = append(cases, testAccResourceTemplate{
		"all fields are loaded correctly",
		localExec,
		resource.ComposeTestCheckFunc(
			resource.TestMatchResourceAttr("enos_local_exec.foo", "id", regexp.MustCompile(`^foo$`)),
			resource.TestMatchResourceAttr("enos_local_exec.foo", "stdout", regexp.MustCompile(`^$`)),
			resource.TestMatchResourceAttr("enos_local_exec.foo", "stderr", regexp.MustCompile(`^$`)),
			resource.TestMatchResourceAttr("enos_local_exec.foo", "environment[FOO]", regexp.MustCompile(`^BAR$`)),
			resource.TestMatchResourceAttr("enos_local_exec.foo", "inline[0]", regexp.MustCompile(`^/tmp/foo$`)),
			resource.TestMatchResourceAttr("enos_local_exec.foo", "scripts[0]", regexp.MustCompile(`^../fixtures/src.txt$`)),
			resource.TestMatchResourceAttr("enos_local_exec.foo", "content", regexp.MustCompile(`^some content$`)),
		),
		false,
	})
	realTest := newLocalExecStateV1()
	realTest.ID = "foo"
	realTest.Env = map[string]string{"FOO": "BAR"}
	realTest.Scripts = []string{"../fixtures/script.sh"}
	realTest.Inline = []string{"touch /tmp/foo && rm /tmp/foo"}
	realTest.Content = `echo "hello world" > /tmp/enos_local_exec_script_content`
	cases = append(cases, testAccResourceTemplate{
		"real test",
		realTest,
		resource.ComposeTestCheckFunc(),
		true,
	})
	noStdoutOrStderr := newLocalExecStateV1()
	noStdoutOrStderr.ID = "foo"
	noStdoutOrStderr.Inline = []string{"exit 0"}
	cases = append(cases, testAccResourceTemplate{
		"NoStdoutOrStderr",
		noStdoutOrStderr,
		resource.ComposeTestCheckFunc(),
		true,
	})
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

	t.Run("CanHandleUpdatedAttributesAndOutput", func(t *testing.T) {
		steps := []resource.TestStep{}

		for _, cmd := range []string{
			"exit 0",
			"echo 'stderr' 1>&2",
			"echo 'stderr' 1>&2",
			"echo 'stdout' && echo 'stderr' 1>&2",
			"echo 'stdout' && echo 'stderr' 1>&2",
			"exit 0",
		} {
			test := newLocalExecStateV1()
			test.ID = "foo"
			test.Content = cmd
			buf := bytes.Buffer{}
			err := cfg.Execute(&buf, test)
			if err != nil {
				t.Fatalf("error executing test template: %s", err.Error())
			}

			steps = append(steps, resource.TestStep{
				Config: buf.String(),
			})
		}

		resource.ParallelTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProviders,
			Steps:                    steps,
		})
	})
}
