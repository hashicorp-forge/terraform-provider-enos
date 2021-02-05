package plugin

import (
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

type errWithDiagnostics struct {
	summary string
	detail  string
	Err     error
}

func (e errWithDiagnostics) Error() string {
	return fmt.Sprintf("%s: %s", e.summary, e.detail)
}

func newErrWithDiagnostics(summary, detail string) error {
	return errWithDiagnostics{
		summary: summary,
		detail:  detail,
	}
}

func wrapErrWithDiagnostics(summary string, detail string, err error) error {
	return errWithDiagnostics{
		summary: summary,
		detail:  detail,
		Err:     err,
	}
}

func errToDiagnostic(err error) *tfprotov5.Diagnostic {
	diag := &tfprotov5.Diagnostic{
		Severity: tfprotov5.DiagnosticSeverityError,
	}

	var diagErr errWithDiagnostics
	if errors.As(err, &diagErr) {
		diag.Summary = diagErr.summary
		diag.Detail = fmt.Sprintf("%s %s", diagErr.detail, diagErr.Err.Error())
	} else {
		diag.Summary = err.Error()
	}

	return diag
}
