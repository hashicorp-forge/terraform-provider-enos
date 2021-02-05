package plugin

import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tftypes"
)

func dynToValue(dyn *tfprotov5.DynamicValue, to tftypes.Type) (tftypes.Value, error) {
	val, err := dyn.Unmarshal(to)
	if err != nil {
		return val, wrapErrWithDiagnostics(
			"Unexpected configuration format",
			"The resource got a configuration that did not match its schema. This may indicate an error in the provider.", err,
		)
	}

	return val, nil
}
