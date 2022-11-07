package plugin

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func AttributePathError(err error, attributeNames ...string) error {
	return buildAttributePath(attributeNames...).NewError(err)
}

// ValidationError creates a tftypes.AttributePathError, with the error message formatted as follows:
// "validation error, <msg>"
// The attributeNames represent the steps in the path to the attribute that failed validation
func ValidationError(msg string, attributeNames ...string) error {
	return AttributePathError(fmt.Errorf("validation error, %s", msg), attributeNames...)
}

func buildAttributePath(attributes ...string) *tftypes.AttributePath {
	attributePath := tftypes.NewAttributePath()
	for _, attr := range attributes {
		attributePath.WithAttributeName(attr)
	}
	return attributePath
}

func ctxToDiagnostic(ctx context.Context) *tfprotov6.Diagnostic {
	return &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityError,
		Summary:  "Error",
		Detail:   fmt.Sprintf("context canceled: %s", ctx.Err()),
	}
}
