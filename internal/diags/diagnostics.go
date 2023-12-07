package diags

import (
	"errors"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// HasErrors checks if any of the provided Diagnostics is of severity Error.
func HasErrors(diags []*tfprotov6.Diagnostic) bool {
	return GetErrorDiagnostic(diags) != nil
}

// GetErrorDiagnostic Gets the first Diagnostic from the provided Diagnostics that is of severity
// Error.
func GetErrorDiagnostic(diags []*tfprotov6.Diagnostic) *tfprotov6.Diagnostic {
	for _, diag := range diags {
		if diag.Severity == tfprotov6.DiagnosticSeverityError {
			return diag
		}
	}

	return nil
}

// ErrToDiagnostic creates a new Diagnostic for the provided error. If the error is of type
// tftypes.AttributePathError the attribute path is added to the diagnostic.
func ErrToDiagnostic(summary string, err error) *tfprotov6.Diagnostic {
	diagnostic := &tfprotov6.Diagnostic{
		Summary:  summary,
		Severity: tfprotov6.DiagnosticSeverityError,
	}

	var attrErr tftypes.AttributePathError
	if errors.As(err, &attrErr) {
		diagnostic.Detail = attrErr.Unwrap().Error()
		diagnostic.Attribute = attrErr.Path
	} else {
		diagnostic.Detail = err.Error()
	}

	return diagnostic
}

// ErrToDiagnostic creates a new Diagnostic for the provided error. If the error is of type
// tftypes.AttributePathError the attribute path is added to the diagnostic.
func ErrToDiagnosticWarn(summary string, err error) *tfprotov6.Diagnostic {
	diagnostic := &tfprotov6.Diagnostic{
		Summary:  summary,
		Severity: tfprotov6.DiagnosticSeverityWarning,
	}

	var attrErr tftypes.AttributePathError
	if errors.As(err, &attrErr) {
		diagnostic.Detail = attrErr.Unwrap().Error()
		diagnostic.Attribute = attrErr.Path
	} else {
		diagnostic.Detail = err.Error()
	}

	return diagnostic
}
