package plugin

import (
	"fmt"
	"math/big"

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

		switch toType := to.(type) {
		case string, *string:
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
		case *tfBool:
			boolVal := &tfBool{}
			err = boolVal.FromTFValue(val)
			if err != nil {
				return vals, err
			}
			*toType = *boolVal
		case *tfNum:
			numVal := &tfNum{}
			err = numVal.FromTFValue(val)
			if err != nil {
				return vals, err
			}
			*toType = *numVal
		case *tfStringSlice:
			ssVal := &tfStringSlice{}
			err = ssVal.FromTFValue(val)
			if err != nil {
				return vals, err
			}
			*toType = *ssVal
		default:
			if !vals[key].IsKnown() || vals[key].IsNull() {
				continue
			}

			err = vals[key].As(to)
			if err != nil {
				return vals, err
			}
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

// tfMarshalDynamicPsuedoTypeObject is for marshaling a dynamic psuedo-type that is
// only a single level deep into an object. Currently this is experimental and only
// supports string and bool attribute types.
func tfMarshalDynamicPsuedoTypeObject(vals map[string]interface{}, optional map[string]struct{}) tftypes.Value {
	if len(vals) == 0 {
		return tftypes.NewValue(tftypes.Object{}, tftypes.UnknownValue)
	}

	tfVals := map[string]tftypes.Value{}
	tfTypes := map[string]tftypes.Type{}

	for key, val := range vals {
		switch t := val.(type) {
		case string:
			tfTypes[key] = tftypes.String
			if val == UnknownString {
				tfVals[key] = UnknownStringValue
			} else {
				tfVals[key] = tftypes.NewValue(tftypes.String, t)
			}
		case tfBool:
			tfTypes[key] = t.TFType()
			tfVals[key] = t.TFValue()
		case tfNum:
			tfTypes[key] = t.TFType()
			tfVals[key] = t.TFValue()
		case tfStringSlice:
			tfTypes[key] = t.TFType()
			tfVals[key] = t.TFValue()
		case tftypes.Value:
			tfTypes[key] = t.Type()
			tfVals[key] = t
		default:
			continue
		}
	}

	return tftypes.NewValue(tftypes.Object{
		AttributeTypes:     tfTypes,
		OptionalAttributes: optional,
	}, tfVals)
}

// tfUnmarshalDynamicPsuedoType is for unmarshaling a dynamic psuedo-type that
// is only a single level deep. Currently this is experimental and only supports
// string and bool types.
func tfUnmarshalDynamicPsuedoType(val tftypes.Value) (map[string]interface{}, error) {
	res := map[string]interface{}{}
	vals := map[string]tftypes.Value{}
	err := val.As(&vals)
	if err != nil {
		return res, err
	}

	for key, val := range vals {
		valType := val.Type()

		if valType.Is(tftypes.String) {
			str := ""
			if val.IsKnown() && !val.IsNull() {
				err = val.As(&str)
				if err != nil {
					return res, err
				}
			} else {
				str = UnknownString
			}
			res[key] = str
		} else if valType.Is(tftypes.Bool) {
			boolVal := &tfBool{}
			err = boolVal.FromTFValue(val)
			if err != nil {
				return res, err
			}
			res[key] = boolVal
		} else if valType.Is(tftypes.Number) {
			numVal := &tfNum{}
			err = numVal.FromTFValue(val)
			if err != nil {
				return res, err
			}
			res[key] = numVal
		} else if valType.Is(tftypes.DynamicPseudoType) && !val.IsKnown() {
			// In cases where we get unknown values, eg: some attribute is
			// set to unknown and we're planning, just set it to the raw
			// tftypes.Value and we'll pass it back later.
			res[key] = val
		} else {
			return res, fmt.Errorf("marshaling of type %s has not been implemented", valType.String())
		}

	}

	return res, nil
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

type tfBool struct {
	unknown bool
	null    bool
	val     bool
}

func (b *tfBool) TFType() tftypes.Type {
	return tftypes.Bool
}

func (b *tfBool) TFValue() tftypes.Value {
	if b.unknown {
		return tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue)
	}

	if b.null {
		return tftypes.NewValue(tftypes.Bool, nil)
	}

	return tftypes.NewValue(tftypes.Bool, b.val)
}

func (b *tfBool) FromTFValue(val tftypes.Value) error {
	switch {
	case val.Equal(tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue)):
		b.unknown = true
	case val.Equal(tftypes.NewValue(tftypes.Bool, nil)):
		b.null = true
	default:
		var bv bool
		err := val.As(&bv)
		if err != nil {
			return err
		}
		b.Set(bv)
	}

	return nil
}

func (b *tfBool) Get() (bool, bool) {
	if b.unknown || b.null {
		return b.val, false
	}

	return b.val, true
}

func (b *tfBool) Value() bool {
	return b.val
}

func (b *tfBool) Set(val bool) {
	b.unknown = false
	b.null = false
	b.val = val
}

type tfNum struct {
	unknown bool
	null    bool
	val     int
}

func (b *tfNum) TFType() tftypes.Type {
	return tftypes.Number
}

func (b *tfNum) TFValue() tftypes.Value {
	if b.unknown {
		return tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)
	}

	if b.null {
		return tftypes.NewValue(tftypes.Number, nil)
	}

	return tftypes.NewValue(tftypes.Number, b.val)
}

func (b *tfNum) FromTFValue(val tftypes.Value) error {
	switch {
	case val.Equal(tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)):
		b.unknown = true
	case val.Equal(tftypes.NewValue(tftypes.Number, nil)):
		b.null = true
	default:
		i := big.Float{}
		err := val.As(&i)
		if err != nil {
			return err
		}
		in, _ := i.Int64()
		b.Set(int(in))
	}

	return nil
}

func (b *tfNum) Get() (int, bool) {
	if b.unknown || b.null {
		return b.val, false
	}

	return b.val, true
}

func (b *tfNum) Value() int {
	return b.val
}

func (b *tfNum) Set(val int) {
	b.unknown = false
	b.null = false
	b.val = val
}

type tfStringSlice struct {
	unknown bool
	null    bool
	val     []string
}

func (b *tfStringSlice) TFType() tftypes.Type {
	return tftypes.List{ElementType: tftypes.String}
}

func (b *tfStringSlice) TFValue() tftypes.Value {
	if b.unknown {
		return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, tftypes.UnknownValue)
	}

	if b.null {
		return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil)
	}

	return tfMarshalStringSlice(b.val)
}

func (b *tfStringSlice) FromTFValue(val tftypes.Value) error {
	switch {
	case val.Equal(tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, tftypes.UnknownValue)):
		b.unknown = true
	case val.Equal(tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil)):
		b.null = true
	default:
		stringVals, err := tfUnmarshalStringSlice(val)
		if err != nil {
			return err
		}

		b.Set(stringVals)
	}

	return nil
}

func (b *tfStringSlice) Get() ([]string, bool) {
	if b.unknown || b.null {
		return b.val, false
	}

	return b.val, true
}

func (b *tfStringSlice) Value() []string {
	return b.val
}

func (b *tfStringSlice) Set(val []string) {
	b.unknown = false
	b.null = false
	b.val = val
}
