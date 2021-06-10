package plugin

import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// UnknownString is a special value we assign to when unmarshaling an unknown
// string value onto types.
const UnknownString = "__XX_TFTYPES_UNKNOWN_STRING"

// UnknownStringValue is an unknown string in the Terraform wire format
var UnknownStringValue = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)

func marshal(state Serializable) (*tfprotov5.DynamicValue, error) {
	dyn, err := tfprotov5.NewDynamicValue(state.Terraform5Type(), state.Terraform5Value())
	if err != nil {
		return &dyn, wrapErrWithDiagnostics(err,
			"Unexpected configuration format",
			"Failed to marshal the state to a Terraform type",
		)
	}

	return &dyn, nil
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

// mapAttributesTo is a helper to ease mapping basic string, bool, and number tftypes.Types to corresponding go values. The val input should be a top-level marshaled tftypes.Object. The props is a property map that dictates which val field to map to the which go value. The value of the prop map should be a pointer to valid value.
func mapAttributesTo(val tftypes.Value, props map[string]interface{}) (map[string]tftypes.Value, error) {
	vals := map[string]tftypes.Value{}
	err := val.As(&vals)
	if err != nil {
		return vals, err
	}

	for key, to := range props {
		val, ok := vals[key]
		if !ok {
			continue
		}

		// Handle cases where a value is going to be given but is unknown
		if val.Equal(UnknownStringValue) {
			toPtr, ok := to.(*string)

			if ok {
				*toPtr = UnknownString
				continue
			}
		}

		if !vals[key].IsKnown() || vals[key].IsNull() {
			continue
		}

		err = vals[key].As(to)
		if err != nil {
			return vals, err
		}
	}

	return vals, nil
}

func tfUnmarshalStringSlice(val tftypes.Value) ([]string, error) {
	strings := []string{}
	vals := []tftypes.Value{}
	err := val.As(&vals)
	if err != nil {
		return strings, err
	}

	str := ""
	for _, strVal := range vals {
		if strVal.IsKnown() && !strVal.IsNull() {
			err = strVal.As(&str)
			if err != nil {
				return strings, err
			}
		} else {
			str = UnknownString
		}

		strings = append(strings, str)
	}

	return strings, nil
}

func tfUnmarshalStringMap(val tftypes.Value) (map[string]string, error) {
	strings := map[string]string{}
	vals := map[string]tftypes.Value{}
	err := val.As(&vals)
	if err != nil {
		return strings, err
	}

	str := ""
	for strKey, strVal := range vals {
		if strVal.IsKnown() && !strVal.IsNull() {
			err = strVal.As(&str)
			if err != nil {
				return strings, err
			}
		} else {
			str = UnknownString
		}

		strings[strKey] = str
	}

	return strings, nil
}

func tfMarshalStringValue(val string) tftypes.Value {
	if val == "" || val == UnknownString {
		return UnknownStringValue
	}

	return tftypes.NewValue(tftypes.String, val)
}

func tfMarshalStringOptionalValue(val string) tftypes.Value {
	if val == "" {
		return tftypes.NewValue(tftypes.String, nil)
	}

	if val == UnknownString {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	}

	return tftypes.NewValue(tftypes.String, val)
}

func tfMarshalStringAllowBlank(val string) tftypes.Value {
	if val == UnknownString {
		return UnknownStringValue
	}

	return tftypes.NewValue(tftypes.String, val)
}

func tfMarshalStringSlice(vals []string) tftypes.Value {
	if len(vals) == 0 {
		return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil)
	}

	values := []tftypes.Value{}
	for _, val := range vals {
		if val == UnknownString {
			values = append(values, UnknownStringValue)
		} else {
			values = append(values, tftypes.NewValue(tftypes.String, val))
		}
	}

	return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, values)
}

func tfMarshalStringMap(vals map[string]string) tftypes.Value {
	if len(vals) == 0 {
		return tftypes.NewValue(tftypes.Map{AttributeType: tftypes.String}, nil)
	}

	values := map[string]tftypes.Value{}
	for key, val := range vals {
		if val == UnknownString {
			values[key] = UnknownStringValue
		} else {
			values[key] = tftypes.NewValue(tftypes.String, val)
		}
	}

	return tftypes.NewValue(tftypes.Map{AttributeType: tftypes.String}, values)
}

func tfStringsSetOrUnknown(args ...string) bool {
	for _, arg := range args {
		if arg != "" {
			return true
		}
	}

	return false
}
