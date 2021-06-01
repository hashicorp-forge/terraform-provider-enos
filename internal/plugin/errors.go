package plugin

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type errWithDiagnostics struct {
	summary    string
	detail     string
	attributes []string
	Err        error
}

func (e errWithDiagnostics) Error() string {
	msg := fmt.Sprintf("%s: %s", e.summary, e.detail)
	if e.Err != nil {
		msg = fmt.Sprintf("%s: %s", msg, e.Err.Error())
	}

	return msg
}

func newErrWithDiagnostics(summary string, detail string, attributes ...string) error {
	return errWithDiagnostics{
		summary:    summary,
		detail:     detail,
		attributes: attributes,
	}
}

func wrapErrWithDiagnostics(err error, summary string, detail string, attributes ...string) error {
	return errWithDiagnostics{
		summary:    summary,
		detail:     detail,
		attributes: attributes,
		Err:        err,
	}
}

func errToDiagnostic(err error) *tfprotov5.Diagnostic {
	diag := &tfprotov5.Diagnostic{
		Severity: tfprotov5.DiagnosticSeverityError,
	}

	var diagErr errWithDiagnostics
	if errors.As(err, &diagErr) {
		diag.Summary = diagErr.summary

		detail := strings.Builder{}
		detail.WriteString(fmt.Sprintf("%s %s", diagErr.detail, diagErr.Error()))

		for {
			err := errors.Unwrap(err)
			if err == nil {
				break
			}
			detail.WriteString(err.Error())
		}
		diag.Detail = detail.String()

		if len(diagErr.attributes) > 0 {
			steps := []tftypes.AttributePathStep{}
			for _, attr := range diagErr.attributes {
				steps = append(steps, tftypes.AttributeName(attr))
			}
			diag.Attribute = tftypes.NewAttributePathWithSteps(steps)
		}
	} else {
		diag.Summary = err.Error()
		detail := strings.Builder{}
		for {
			err = errors.Unwrap(err)
			if err == nil {
				break
			}
			detail.WriteString(err.Error())
		}
		diag.Detail = detail.String()
	}

	return diag
}
