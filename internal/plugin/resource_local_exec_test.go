package plugin

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccResourceLocalExec tests the local_exec resource.
func TestAccResourceLocalExec(t *testing.T) {
	t.Parallel()

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
	//nolint:paralleltest// because our resource handles it
	for _, test := range cases {
		test := test
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
			ProtoV6ProviderFactories: testProviders(t),
			Steps:                    steps,
		})
	})
}

//nolint:paralleltest// because our resource handles it
func TestResourceReAppliedWhenEnvChanges(t *testing.T) {
	tempDir := t.TempDir() // Note: this dir is automatically deleted after the test is run
	f, err := os.CreateTemp(tempDir, "reapply_test.txt")
	assert.NoError(t, err)

	cfg := template.Must(template.New("enos_local_exec").Parse(`resource "enos_local_exec" "reapply_test" {
        content = "echo \"hello\" >> {{ .File }}"

        environment = {
        {{range $name, $val := .Env}}
            "{{$name}}": "{{$val}}",
        {{end}}
        }
    }`))

	data1 := map[string]interface{}{
		"File": f.Name(),
		"Env": map[string]string{
			"one": "one",
		},
	}

	s1 := bytes.Buffer{}
	err = cfg.Execute(&s1, data1)
	if err != nil {
		t.Fatalf("error executing test template: %s", err.Error())
	}

	apply1 := resource.TestStep{
		Config:   s1.String(),
		PlanOnly: false,
	}

	data2 := map[string]interface{}{
		"File": f.Name(),
		"Env": map[string]string{
			"one": "one",
			"two": "one",
		},
	}

	s2 := bytes.Buffer{}
	err = cfg.Execute(&s2, data2)
	if err != nil {
		t.Fatalf("error executing test template: %s", err.Error())
	}

	apply2 := resource.TestStep{
		Config:   s2.String(),
		PlanOnly: false,
	}

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviders(t),
		Steps: []resource.TestStep{
			apply1,
			apply2,
		},
	})

	actual, err := io.ReadAll(f)
	assert.NoError(t, err)
	assert.Equal(t, "hello\nhello\n", string(actual))
}
