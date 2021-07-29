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
	cfg := template.Must(template.New("enos_local_exec").Parse(`resource "enos_local_exec" "{{.ID.Value}}" {
        {{if .Content.Value}}
        content = <<EOF
{{.Content.Value}}
EOF
        {{end}}
        {{if .Inline.StringValue}}
        inline = [
        {{range .Inline.StringValue}}
            "{{.}}",
        {{end}}
        ]
        {{end}}
        {{if .Scripts.StringValue}}
        scripts = [
        {{range .Scripts.StringValue}}
            "{{.}}",
        {{end}}
        ]
        {{end}}
        {{if .Env.StringValue}}
        environment = {
        {{range $name, $val := .Env.StringValue}}
            "{{$name}}": "{{$val}}",
        {{end}}
        }
        {{end}}
    }`))
	cases := []testAccResourceTemplate{}
	localExec := newLocalExecStateV1()
	localExec.ID.Set("foo")
	localExec.Env.SetStrings(map[string]string{"FOO": "BAR"})
	localExec.Scripts.SetStrings([]string{"../fixtures/src.txt"})
	localExec.Inline.SetStrings([]string{"touch /tmp/foo"})
	localExec.Content.Set("some content")
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
	realTest.ID.Set("foo")
	realTest.Env.SetStrings(map[string]string{"FOO": "BAR"})
	realTest.Scripts.SetStrings([]string{"../fixtures/script.sh"})
	realTest.Inline.SetStrings([]string{"touch /tmp/foo && rm /tmp/foo"})
	realTest.Content.Set(`echo "hello world" > /tmp/enos_local_exec_script_content`)
	cases = append(cases, testAccResourceTemplate{
		"real test",
		realTest,
		resource.ComposeTestCheckFunc(),
		true,
	})
	noStdoutOrStderr := newLocalExecStateV1()
	noStdoutOrStderr.ID.Set("foo")
	noStdoutOrStderr.Inline.SetStrings([]string{"exit 0"})
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
			test.ID.Set("foo")
			test.Content.Set(cmd)
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
