package plugin

import (
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
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

func (e errWithDiagnostics) Unwrap() error {
	return e.Err
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

// errToDiagnostic takes and error and returns the innermost diagnostic error
// If a diagnostic error isn't found in the chain then an aggregate chain
// will be returned.
func errToDiagnostic(err error) *tfprotov6.Diagnostic {
	diag := &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityError,
		Summary:  "Error",
		Detail:   err.Error(),
	}

	appendErrToDiag := func(diag *tfprotov6.Diagnostic, err error) {
		var diagErr errWithDiagnostics
		if errors.As(err, &diagErr) {
			diag.Summary = diagErr.summary
			diag.Detail = diagErr.detail
			if len(diagErr.attributes) > 0 {
				steps := []tftypes.AttributePathStep{}
				for _, attr := range diagErr.attributes {
					steps = append(steps, tftypes.AttributeName(attr))
				}
				diag.Attribute = tftypes.NewAttributePathWithSteps(steps)
			}
		}
	}

	// Go through the entire error chain to build a diagnostic
	for {
		if err == nil {
			break
		}
		appendErrToDiag(diag, err)
		err = errors.Unwrap(err)
	}

	return diag
}
