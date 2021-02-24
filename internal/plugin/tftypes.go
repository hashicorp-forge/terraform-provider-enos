package plugin

import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tftypes"
)

func marshal(state Serializable) (*tfprotov5.DynamicValue, error) {
	dyn, err := tfprotov5.NewDynamicValue(state.Terraform5Type(), state.Terraform5Value())

	return &dyn, err
}

func unmarshal(state Serializable, dyn *tfprotov5.DynamicValue) error {
	val, err := dyn.Unmarshal(state.Terraform5Type())
	if err != nil {
		return wrapErrWithDiagnostics(err,
			"Unexpected configuration format",
			"The resource got a configuration that did not match its schema. This may indicate an error in the provider.",
		)
	}

	err = val.As(state)
	if err != nil {
		return wrapErrWithDiagnostics(err,
			"Unexpected configuration format",
			"The resource state implementation does not match its schema. This may indicate an error in the provider.",
		)
	}

	return nil
}

func marshalDelete(state Serializable) (*tfprotov5.DynamicValue, error) {
	dyn, err := tfprotov5.NewDynamicValue(state.Terraform5Type(), tftypes.NewValue(state.Terraform5Type(), nil))
	if err != nil {
		err = wrapErrWithDiagnostics(err,
			"Unexpected configuration format",
			"The resource state implementation does not match its schema. This may indicate an error in the provider.",
		)
	}

	return &dyn, err
}

// upgradeState takes an existing state and the new values we're migrating.
// It unmarshals the new values onto the current state and returns a new
// marshaled upgraded state.
func upgradeState(currentState Serializable, newValues tftypes.Value) (*tfprotov5.DynamicValue, error) {
	upgraded, err := tfprotov5.NewDynamicValue(currentState.Terraform5Type(), newValues)
	if err != nil {
		return &upgraded, wrapErrWithDiagnostics(
			err,
			"upgrade error",
			"unable to map version 1 to the current state",
		)
	}

	// Apply the new values to current state
	err = unmarshal(currentState, &upgraded)
	if err != nil {
		return &upgraded, wrapErrWithDiagnostics(
			err,
			"upgrade error",
			"unable to apply upgraded values to state",
		)
	}

	// Return the current state in the wire format
	return marshal(currentState)
}

func mapAttributeTo(vals map[string]tftypes.Value, key string, to interface{}) error {
	if vals[key].IsKnown() && !vals[key].IsNull() {
		return vals[key].As(to)
	}

	return nil
}

func mapAttributesTo(val tftypes.Value, props map[string]interface{}) (map[string]tftypes.Value, error) {
	vals := map[string]tftypes.Value{}
	err := val.As(&vals)
	if err != nil {
		return vals, err
	}

	for key, to := range props {
		err := mapAttributeTo(vals, key, to)
		if err != nil {
			return vals, err
		}
	}

	return vals, nil
}

func stringValue(val string) tftypes.Value {
	if val == "" {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	}

	return tftypes.NewValue(tftypes.String, val)
}
