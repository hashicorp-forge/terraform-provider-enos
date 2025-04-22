// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"
	"math/big"
	"reflect"
	"regexp"
	"strconv"

	"github.com/pkg/errors"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/server/state"
)

var (
	impliedTypeRegexp = regexp.MustCompile(`\d*?\[\"(\w*)\",.*]`)
	nullDSTVal        = tftypes.NewValue(tftypes.DynamicPseudoType, nil)
	unknownDSTVal     = tftypes.NewValue(tftypes.DynamicPseudoType, tftypes.UnknownValue)
)

func unmarshal(state state.Serializable, dyn *tfprotov6.DynamicValue) error {
	val, err := dyn.Unmarshal(state.Terraform5Type())
	if err != nil {
		return fmt.Errorf("failed to unmarshal Terraform value, this may indicate an error in the provider, cause: %w", err)
	}

	err = val.As(state)
	if err != nil {
		return fmt.Errorf("failed to unmarshal Terraform value, configuration does not match its schema, this may indicate an error in the provider, cause: %w", err)
	}

	return nil
}

// upgradeState takes an existing state and the new values we're migrating.
// It unmarshals the new values onto the current state and returns a new
// marshaled upgraded state.
func upgradeState(currentState state.Serializable, newValues tftypes.Value) (*tfprotov6.DynamicValue, error) {
	upgraded, err := tfprotov6.NewDynamicValue(currentState.Terraform5Type(), newValues)
	if err != nil {
		return &upgraded, fmt.Errorf("failed to upgrade state, unable to map version 1 to the current state, due to: %w", err)
	}

	// Apply the new values to current state
	err = unmarshal(currentState, &upgraded)
	if err != nil {
		return &upgraded, fmt.Errorf("failed to upgrade state, unable to apply upgraded values to state, due to: %w", err)
	}

	// Return the current state in the wire format
	return state.Marshal(currentState)
}

// mapAttributesTo is a helper that maps tftypes.Values into intermediary
// types that are easier to use in providers. The val input should be a top-level
// marshaled tftypes.Value. The props is a property map that dictates which val
// field to map to the which go value. The value of the prop map should be a
// pointer to valid value.
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

		tfType, ok := to.(TFType)
		if ok {
			err = tfType.FromTFValue(val)
		} else {
			if !vals[key].IsKnown() || vals[key].IsNull() {
				continue
			}

			err = vals[key].As(to)
		}

		if err != nil {
			return vals, err
		}
	}

	return vals, nil
}

func tfStringsSetOrUnknown(args ...*tfString) bool {
	for _, arg := range args {
		if arg.Unknown {
			return true
		}

		_, ok := arg.Get()
		if ok {
			return true
		}
	}

	return false
}

// Sit down, grab a beverage, lets tell a story.
//
// Consider what we have to do when we're dealing with DynamicPseudoType's and Terraform's strict
// type checking. When Terraform creates a dynamic object during plan/apply it will send the object
// over the wire as either a map or an object. All good except that tftypes does not expose this
// information to us but we need to correctly marshal the tfObject to the correct tftypes.Value,
// otherwise Terraform will crash. The only place I could find this type information is by taking
// the planned tftypes.Value and marshaling it to the wire format to inspect the hidden type
// information Terraform sent over the wire, and then using it to create our new value with the
// correct expected type.
//
// This is terrible but had to be done until better support for DynamicPseudoType's as input schema
// is added to terraform-plugin-go.
func encodeTfObjectDynamicPseudoType(
	planVal tftypes.Value,
	objVals map[string]tftypes.Value,
) (tftypes.Value, error) {
	var res tftypes.Value

	// MarshalMsgPack is deprecated but it's by far the easiest way to inspect the serialized value
	// of the raw attribute.
	//nolint:staticcheck,nolintlint
	//lint:ignore SA1019 we have to use this internal only API to determine DynamicPseudoType types.
	msgpackBytes, err := planVal.MarshalMsgPack(tftypes.DynamicPseudoType)
	if err != nil {
		return res, errors.Wrap(err, "unable to marshal the vault config block to the wire format")
	}

	matches := impliedTypeRegexp.FindStringSubmatch(string(msgpackBytes))
	if len(matches) >= 1 {
		switch matches[1] {
		case "map":
			var elemType tftypes.Type
			for _, attr := range objVals {
				elemType = attr.Type()
				break
			}

			return tftypes.NewValue(tftypes.Map{ElementType: elemType}, objVals), nil
		case "object":
			return terraform5Value(objVals), nil
		default:
			return res, errors.New(matches[1] + " is not a support dynamic type for the vault config block")
		}
	}

	return res, fmt.Errorf("unable to determine type, got matches: %v+, in bytes: %s", matches, msgpackBytes)
}

func newTfBool() *tfBool {
	return &tfBool{Null: true}
}

type tfBool struct {
	Unknown bool
	Null    bool
	Val     bool
}

var _ TFType = (*tfBool)(nil)

func (b *tfBool) TFType() tftypes.Type {
	return tftypes.Bool
}

func (b *tfBool) TFValue() tftypes.Value {
	if b.Unknown {
		return tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue)
	}

	if b.Null {
		return tftypes.NewValue(tftypes.Bool, nil)
	}

	return tftypes.NewValue(tftypes.Bool, b.Val)
}

func (b *tfBool) FromTFValue(val tftypes.Value) error {
	switch {
	case val.Equal(unknownDSTVal), val.Equal(tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue)):
		b.Unknown = true
	case val.Equal(nullDSTVal), val.Equal(tftypes.NewValue(tftypes.Bool, nil)):
		b.Null = true
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
	if b.Unknown || b.Null {
		return b.Val, false
	}

	return b.Val, true
}

func (b *tfBool) Value() bool {
	return b.Val
}

func (b *tfBool) Set(val bool) {
	b.Unknown = false
	b.Null = false
	b.Val = val
}

func (b *tfBool) Eq(o *tfBool) bool {
	if o == nil {
		return false
	}

	return reflect.DeepEqual(b, o)
}

func (b *tfBool) String() string {
	switch {
	case b.Unknown:
		return "unknown"
	case b.Null:
		return "null"
	default:
		return strconv.FormatBool(b.Val)
	}
}

func newTfNum() *tfNum {
	return &tfNum{Null: true}
}

type tfNum struct {
	Unknown bool
	Null    bool
	Val     int
}

var _ TFType = (*tfNum)(nil)

func (b *tfNum) TFType() tftypes.Type {
	return tftypes.Number
}

func (b *tfNum) TFValue() tftypes.Value {
	if b.Unknown {
		return tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)
	}

	if b.Null {
		return tftypes.NewValue(tftypes.Number, nil)
	}

	return tftypes.NewValue(tftypes.Number, b.Val)
}

func (b *tfNum) FromTFValue(val tftypes.Value) error {
	switch {
	case val.Equal(unknownDSTVal), val.Equal(tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)):
		b.Unknown = true
	case val.Equal(nullDSTVal), val.Equal(tftypes.NewValue(tftypes.Number, nil)):
		b.Null = true
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
	if b.Unknown || b.Null {
		return b.Val, false
	}

	return b.Val, true
}

func (b *tfNum) Value() int {
	return b.Val
}

func (b *tfNum) Set(val int) {
	b.Unknown = false
	b.Null = false
	b.Val = val
}

func (b *tfNum) Eq(o *tfNum) bool {
	if o == nil {
		return false
	}

	return reflect.DeepEqual(b, o)
}

func (b *tfNum) String() string {
	switch {
	case b.Unknown:
		return "unknown"
	case b.Null:
		return "null"
	default:
		return strconv.Itoa(b.Val)
	}
}

func newTfString() *tfString {
	return &tfString{Null: true}
}

type tfString struct {
	Unknown bool
	Null    bool
	Val     string
}

var _ TFType = (*tfString)(nil)

func (b *tfString) TFType() tftypes.Type {
	return tftypes.String
}

func (b *tfString) TFValue() tftypes.Value {
	if b.Unknown {
		return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	}

	if b.Null {
		return tftypes.NewValue(tftypes.String, nil)
	}

	return tftypes.NewValue(tftypes.String, b.Val)
}

func (b *tfString) FromTFValue(val tftypes.Value) error {
	switch {
	case val.Equal(unknownDSTVal), val.Equal(tftypes.NewValue(tftypes.String, tftypes.UnknownValue)):
		b.Unknown = true
	case val.Equal(nullDSTVal), val.Equal(tftypes.NewValue(tftypes.String, nil)):
		b.Null = true
	default:
		var sv string
		err := val.As(&sv)
		if err != nil {
			return err
		}
		b.Set(sv)
	}

	return nil
}

func (b *tfString) Get() (string, bool) {
	if b.Unknown || b.Null {
		return b.Val, false
	}

	return b.Val, true
}

func (b *tfString) Value() string {
	return b.Val
}

func (b *tfString) Set(val string) {
	b.Unknown = false
	b.Null = false
	b.Val = val
}

func (b *tfString) Eq(o *tfString) bool {
	if o == nil {
		return false
	}

	return reflect.DeepEqual(b, o)
}

func (b *tfString) String() string {
	switch {
	case b.Unknown:
		return "unknown"
	case b.Null:
		return "null"
	default:
		return b.Val
	}
}

func newTfStringSlice() *tfStringSlice {
	return &tfStringSlice{
		Null: true,
		Val:  []*tfString{},
	}
}

type tfStringSlice struct {
	Unknown bool
	Null    bool
	Val     []*tfString
}

var _ TFType = (*tfStringSlice)(nil)

func (b *tfStringSlice) TFType() tftypes.Type {
	return tftypes.List{ElementType: tftypes.String}
}

func (b *tfStringSlice) TFValue() tftypes.Value {
	if b.Unknown {
		return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, tftypes.UnknownValue)
	}

	if b.Null {
		return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil)
	}

	if len(b.Val) == 0 {
		return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil)
	}

	values := []tftypes.Value{}
	for _, val := range b.Val {
		values = append(values, val.TFValue())
	}

	return tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, values)
}

func (b *tfStringSlice) FromTFValue(val tftypes.Value) error {
	switch {
	case val.Equal(unknownDSTVal), val.Equal(tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, tftypes.UnknownValue)):
		b.Unknown = true
	case val.Equal(nullDSTVal), val.Equal(tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil)):
		b.Null = true
	default:
		strs := []*tfString{}
		vals := []tftypes.Value{}
		err := val.As(&vals)
		if err != nil {
			return err
		}

		for _, v := range vals {
			str := newTfString()
			err = str.FromTFValue(v)
			if err != nil {
				return err
			}

			strs = append(strs, str)
		}

		b.Set(strs)
	}

	return nil
}

func (b *tfStringSlice) Get() ([]*tfString, bool) {
	if b.Unknown || b.Null {
		return b.Val, false
	}

	return b.Val, true
}

func (b *tfStringSlice) GetStrings() ([]string, bool) {
	res := []string{}
	strs, ok := b.Get()
	if !ok {
		return res, ok
	}

	for _, str := range strs {
		v, ok := str.Get()
		if !ok {
			return res, ok
		}

		res = append(res, v)
	}

	return res, true
}

func (b *tfStringSlice) Value() []*tfString {
	return b.Val
}

func (b *tfStringSlice) StringValue() []string {
	strs, _ := b.GetStrings()
	return strs
}

func (b *tfStringSlice) Set(val []*tfString) {
	b.Unknown = false
	b.Null = false
	b.Val = val
}

func (b *tfStringSlice) SetStrings(strs []string) {
	b.Unknown = false
	b.Null = false
	tfStrs := []*tfString{}
	for _, str := range strs {
		strVal := newTfString()
		strVal.Set(str)
		tfStrs = append(tfStrs, strVal)
	}
	b.Set(tfStrs)
}

func (b *tfStringSlice) Eq(o *tfStringSlice) bool {
	if o == nil {
		return false
	}

	return reflect.DeepEqual(b, o)
}

func (b *tfStringSlice) FullyKnown() bool {
	strs, ok := b.Get()
	if !ok {
		return false
	}

	for _, str := range strs {
		_, ok := str.Get()
		if !ok {
			return false
		}
	}

	return true
}

func (b *tfStringSlice) String() string {
	switch {
	case b.Unknown:
		return "unknown"
	case b.Null:
		return "null"
	default:
		return fmt.Sprintf("%s", b.Val)
	}
}

func newTfStringMap() *tfStringMap {
	return &tfStringMap{
		Null: true,
		Val:  map[string]*tfString{},
	}
}

type tfStringMap struct {
	Unknown bool
	Null    bool
	Val     map[string]*tfString
}

var _ TFType = (*tfStringMap)(nil)

func (b *tfStringMap) TFType() tftypes.Type {
	return tftypes.Map{ElementType: tftypes.String}
}

func (b *tfStringMap) TFValue() tftypes.Value {
	if b.Unknown {
		return tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, tftypes.UnknownValue)
	}

	if b.Null {
		return tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil)
	}

	if len(b.Val) == 0 {
		return tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil)
	}

	values := map[string]tftypes.Value{}
	for key, val := range b.Val {
		values[key] = val.TFValue()
	}

	return tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, values)
}

func (b *tfStringMap) FromTFValue(val tftypes.Value) error {
	switch {
	case val.Equal(unknownDSTVal), val.Equal(tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, tftypes.UnknownValue)):
		b.Unknown = true
	case val.Equal(nullDSTVal), val.Equal(tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil)):
		b.Null = true
	default:
		strs := map[string]*tfString{}
		vals := map[string]tftypes.Value{}
		err := val.As(&vals)
		if err != nil {
			return err
		}

		for k, v := range vals {
			str := newTfString()
			err = str.FromTFValue(v)
			if err != nil {
				return err
			}

			strs[k] = str
		}

		b.Set(strs)
	}

	return nil
}

func (b *tfStringMap) Get() (map[string]*tfString, bool) {
	if b.Unknown || b.Null {
		return b.Val, false
	}

	return b.Val, true
}

func (b *tfStringMap) GetStrings() (map[string]string, bool) {
	res := map[string]string{}
	strs, ok := b.Get()
	if !ok {
		return res, ok
	}

	for key, str := range strs {
		v, ok := str.Get()
		if !ok {
			return res, ok
		}

		res[key] = v
	}

	return res, true
}

func (b *tfStringMap) Value() map[string]*tfString {
	return b.Val
}

func (b *tfStringMap) StringValue() map[string]string {
	strs, _ := b.GetStrings()
	return strs
}

func (b *tfStringMap) Set(strs map[string]*tfString) {
	b.Unknown = false
	b.Null = false
	b.Val = strs
}

func (b *tfStringMap) SetStrings(strs map[string]string) {
	b.Unknown = false
	b.Null = false
	tfStrs := map[string]*tfString{}
	for k, v := range strs {
		strVal := newTfString()
		strVal.Set(v)
		tfStrs[k] = strVal
	}
	b.Set(tfStrs)
}

func (b *tfStringMap) Eq(o *tfStringMap) bool {
	if o == nil {
		return false
	}

	return reflect.DeepEqual(b, o)
}

func (b *tfStringMap) FullyKnown() bool {
	val, ok := b.Get()
	if !ok {
		return false
	}

	for _, v := range val {
		_, ok := v.Get()
		if !ok {
			return false
		}
	}

	return true
}

func (b *tfStringMap) String() string {
	switch {
	case b.Unknown:
		return "unknown"
	case b.Null:
		return "null"
	default:
		return fmt.Sprintf("%s", b.Val)
	}
}

func newTfObject() *tfObject {
	return &tfObject{
		Null:      true,
		AttrTypes: map[string]tftypes.Type{},
		Val:       map[string]interface{}{},
		Optional:  map[string]struct{}{},
	}
}

type tfObject struct {
	Unknown   bool
	Null      bool
	AttrTypes map[string]tftypes.Type
	Val       map[string]interface{}
	Optional  map[string]struct{}
}

var _ TFType = (*tfObject)(nil)

// FullyKnown returns a boolean of whether or not all values are fully known. If callers are using
// tfObject as a DynamicPseudoType they can call this to determine whether or not we should return
// a DynamicPseudoType or real type.
func (b *tfObject) FullyKnown() bool {
	if b == nil {
		return false
	}

	if b.Unknown {
		return false
	}

	for _, val := range b.Val {
		tfType, ok := val.(TFType)
		if ok {
			if !tfType.TFValue().IsFullyKnown() {
				return false
			}

			continue
		}
		tfVal, ok := val.(tftypes.Value)
		if ok {
			if !tfVal.IsFullyKnown() {
				return false
			}
		}
	}

	return true
}

func (b *tfObject) TFType() tftypes.Type {
	// Sometimes objects represent DynamicPseudoType's. In this cases we want
	// to iterate over the Val if any and ensure that we include them in the type.
	for key, val := range b.Val {
		tfType, ok := val.(TFType)
		if ok {
			b.AttrTypes[key] = tfType.TFType()
		} else {
			switch t := val.(type) {
			case tftypes.Value:
				b.AttrTypes[key] = t.Type()
			default:
				panic(fmt.Sprintf("AttrType(%s) not supported in tfObject. Did you forget to convert from a go type to a *tf* type?", t))
			}
		}
	}

	return tftypes.Object{
		AttributeTypes:     b.AttrTypes,
		OptionalAttributes: b.Optional,
	}
}

func (b *tfObject) TFValue() tftypes.Value {
	tfVals := map[string]tftypes.Value{}

	for key, val := range b.Val {
		tfType, ok := val.(TFType)
		if ok {
			tfVals[key] = tfType.TFValue()
		} else {
			switch t := val.(type) {
			case tftypes.Value:
				tfVals[key] = t
			default:
				panic(fmt.Sprintf("AttrType(%s) not supported in tfObject. Did you forget to convert from a go type to a *tf* type?", t))
			}
		}
	}

	if b.Unknown {
		return tftypes.NewValue(b.TFType(), tftypes.UnknownValue)
	}

	if b.Null || len(b.Val) == 0 {
		return tftypes.NewValue(b.TFType(), nil)
	}

	return tftypes.NewValue(b.TFType(), tfVals)
}

func (b *tfObject) FromTFValue(val tftypes.Value) error {
	switch {
	case val.Equal(unknownDSTVal), val.Equal(tftypes.NewValue(tftypes.Object{}, tftypes.UnknownValue)), !val.IsKnown():
		b.Unknown = true
	case val.Equal(nullDSTVal), val.Equal(tftypes.NewValue(tftypes.DynamicPseudoType, nil)):
		b.Null = true
	default:
		res := map[string]interface{}{}
		vals := map[string]tftypes.Value{}
		err := val.As(&vals)
		if err != nil {
			return err
		}

		for key, val := range vals {
			valType := val.Type()
			switch {
			case valType.Is(tftypes.Bool):
				boolVal := newTfBool()
				err = boolVal.FromTFValue(val)
				if err != nil {
					return err
				}
				res[key] = boolVal
			case valType.Is(tftypes.String):
				strVal := newTfString()
				err = strVal.FromTFValue(val)
				if err != nil {
					return err
				}
				res[key] = strVal
			case valType.Is(tftypes.Number):
				numVal := newTfNum()
				err = numVal.FromTFValue(val)
				if err != nil {
					return err
				}
				res[key] = numVal
			case valType.Is(tftypes.List{ElementType: tftypes.String}):
				listVal := newTfStringSlice()
				err = listVal.FromTFValue(val)
				if err != nil {
					return err
				}
				res[key] = listVal
			case valType.Is(tftypes.Map{ElementType: tftypes.String}):
				mapVal := newTfStringMap()
				err = mapVal.FromTFValue(val)
				if err != nil {
					return err
				}
				res[key] = mapVal
			case valType.Is(tftypes.DynamicPseudoType) && !val.IsKnown():
				// In cases where we get Unknown Values, eg: some attribute is
				// set to Unknown and we're planning, just set it to the raw
				// tftypes.Value and we'll pass it back later.
				res[key] = val
			default:
				// We can't really use `Is()` or `Equal()` for types that include
				// object since the AttributeTypes and OptionalAttributes have
				// to be equal in order for that to match. So instead we'll
				// cast here.
				_, okObj := valType.(tftypes.Object)
				l, okList := valType.(tftypes.List)
				if okList {
					_, okList = l.ElementType.(tftypes.Object)
				}

				if okObj || okList {
					objVal := newTfObject()
					err = objVal.FromTFValue(val)
					if err != nil {
						return err
					}
					res[key] = objVal

					continue
				}

				return fmt.Errorf("marshaling of type %s has not been implemented", valType.String())
			}
		}

		b.Set(res)
	}

	return nil
}

// Get will return the object as map of tf* types.
func (b *tfObject) Get() (map[string]interface{}, bool) {
	if b.Unknown || b.Null {
		return b.Val, false
	}

	return b.Val, true
}

// GetObject will return the object as a map of native go types.
func (b *tfObject) GetObject() (map[string]interface{}, bool) {
	objs := map[string]interface{}{}

	tfAttrs, ok := b.Get()
	if !ok {
		return objs, false
	}

	for key, obj := range tfAttrs {
		switch t := obj.(type) {
		case *tfString:
			if str, ok := t.Get(); ok {
				objs[key] = str
			} else {
				return map[string]interface{}{}, false
			}
		case *tfNum:
			if num, ok := t.Get(); ok {
				objs[key] = num
			} else {
				return map[string]interface{}{}, false
			}
		case *tfBool:
			if b, ok := t.Get(); ok {
				objs[key] = b
			} else {
				return map[string]interface{}{}, false
			}
		case *tfStringSlice:
			if strs, ok := t.GetStrings(); ok {
				objs[key] = strs
			} else {
				return map[string]interface{}{}, false
			}
		case *tfStringMap:
			if strs, ok := t.GetStrings(); ok {
				objs[key] = strs
			} else {
				return map[string]interface{}{}, false
			}
		case *tfObject:
			if obj, ok := t.GetObject(); ok {
				objs[key] = obj
			} else {
				return map[string]interface{}{}, false
			}
		case *tfObjectSlice:
			if obj, ok := t.GetObjects(); ok {
				objs[key] = obj
			} else {
				return map[string]interface{}{}, false
			}
		default:
			objs[key] = obj
		}
	}

	return objs, true
}

func (b *tfObject) Value() map[string]interface{} {
	return b.Val
}

func (b *tfObject) Set(obj map[string]interface{}) {
	b.Unknown = false
	b.Null = false
	b.Val = obj
}

func (b *tfObject) Eq(o *tfObject) bool {
	if o == nil {
		return false
	}

	return reflect.DeepEqual(b, o)
}

func (b *tfObject) String() string {
	switch {
	case b.Unknown:
		return "unknown"
	case b.Null:
		return "null"
	default:
		return fmt.Sprintf("%v", b.Val)
	}
}

func newTfObjectSlice() *tfObjectSlice {
	return &tfObjectSlice{
		Null:      true,
		Val:       []*tfObject{},
		AttrTypes: map[string]tftypes.Type{},
		Optional:  map[string]struct{}{},
	}
}

type tfObjectSlice struct {
	Unknown   bool
	Null      bool
	Val       []*tfObject
	AttrTypes map[string]tftypes.Type
	Optional  map[string]struct{}
}

var _ TFType = (*tfObjectSlice)(nil)

func (b *tfObjectSlice) TFType() tftypes.Type {
	return tftypes.List{ElementType: tftypes.Object{
		AttributeTypes:     b.AttrTypes,
		OptionalAttributes: b.Optional,
	}}
}

func (b *tfObjectSlice) TFValue() tftypes.Value {
	if b.Unknown {
		return tftypes.NewValue(b.TFType(), tftypes.UnknownValue)
	}

	if b.Null || len(b.Val) == 0 {
		return tftypes.NewValue(b.TFType(), nil)
	}

	values := []tftypes.Value{}
	for _, val := range b.Val {
		values = append(values, val.TFValue())
	}

	return tftypes.NewValue(b.TFType(), values)
}

func (b *tfObjectSlice) FromTFValue(val tftypes.Value) error {
	switch {
	case val.Equal(unknownDSTVal), val.Equal(tftypes.NewValue(b.TFType(), tftypes.UnknownValue)):
		b.Unknown = true
	case val.Equal(nullDSTVal), val.Equal(tftypes.NewValue(b.TFType(), nil)):
		b.Null = true
	default:
		objs := []*tfObject{}
		vals := []tftypes.Value{}
		err := val.As(&vals)
		if err != nil {
			return err
		}

		for _, val := range vals {
			obj := newTfObject()
			obj.AttrTypes = b.AttrTypes
			obj.Optional = b.Optional
			err = obj.FromTFValue(val)
			if err != nil {
				return err
			}

			objs = append(objs, obj)
		}

		b.Set(objs)
	}

	return nil
}

func (b *tfObjectSlice) Get() ([]*tfObject, bool) {
	if b.Unknown || b.Null {
		return b.Val, false
	}

	return b.Val, true
}

func (b *tfObjectSlice) GetObjects() ([]map[string]interface{}, bool) {
	res := []map[string]interface{}{}
	objs, ok := b.Get()
	if !ok {
		return res, ok
	}

	for _, obj := range objs {
		v, ok := obj.GetObject()
		if !ok {
			return res, ok
		}

		res = append(res, v)
	}

	return res, true
}

func (b *tfObjectSlice) Value() []*tfObject {
	return b.Val
}

func (b *tfObjectSlice) ObjectsValue() []map[string]interface{} {
	obs, _ := b.GetObjects()
	return obs
}

func (b *tfObjectSlice) Set(objs []*tfObject) {
	b.Unknown = false
	b.Null = false
	b.Val = objs
}

func (b *tfObjectSlice) SetObjects(objs []map[string]interface{}) {
	b.Unknown = false
	b.Null = false
	tfObjs := []*tfObject{}
	for _, obj := range objs {
		objVal := newTfObject()
		objVal.AttrTypes = b.AttrTypes
		objVal.Optional = b.Optional
		objVal.Set(obj)
		tfObjs = append(tfObjs, objVal)
	}
	b.Set(tfObjs)
}

func (b *tfObjectSlice) Eq(o *tfObjectSlice) bool {
	if o == nil {
		return false
	}

	return reflect.DeepEqual(b, o)
}

func (b *tfObjectSlice) FullyKnown() bool {
	objs, ok := b.Get()
	if !ok {
		return false
	}

	for _, obj := range objs {
		_, ok := obj.Get()
		if !ok {
			return false
		}
	}

	return true
}

func (b *tfObjectSlice) String() string {
	switch {
	case b.Unknown:
		return "unknown"
	case b.Null:
		return "null"
	default:
		return fmt.Sprintf("%s", b.Val)
	}
}

type dynamicPseudoTypeBlock struct {
	Object        *tfObject
	OriginalValue tftypes.Value
}

func newDynamicPseudoTypeBlock() *dynamicPseudoTypeBlock {
	return &dynamicPseudoTypeBlock{Object: newTfObject()}
}

// FromTFValue returns unmarshals a tftypes.Value into itself.
func (d *dynamicPseudoTypeBlock) FromTFValue(value tftypes.Value) error {
	if !value.IsKnown() {
		// RetryJoin are a DynamicPseudoType but the value is unknown. Terraform expects us to be a
		// dynamic value that we'll know after apply.
		d.Object.Unknown = true
		return nil
	}
	if value.IsNull() {
		// We can't unmarshal null or unknown things
		return nil
	}

	d.OriginalValue = value

	return d.Object.FromTFValue(value)
}

func (d *dynamicPseudoTypeBlock) TFType() tftypes.Type {
	return tftypes.DynamicPseudoType
}

// TFValue returns marshals the dynamicPseudoTypeBlock into a tftypes.Value.
func (d *dynamicPseudoTypeBlock) TFValue() (tftypes.Value, error) {
	if d == nil || d.Object == nil {
		return tftypes.NewValue(tftypes.DynamicPseudoType, nil), nil
	}

	if d.OriginalValue.Type() == nil {
		// We don't have a type, which means we're a DynamicPseudoType with either a nil or unknown
		// value.
		if d.Object.Unknown {
			return tftypes.NewValue(tftypes.DynamicPseudoType, tftypes.UnknownValue), nil
		}

		return tftypes.NewValue(tftypes.DynamicPseudoType, nil), nil
	}

	values := map[string]tftypes.Value{}
	err := d.Object.TFValue().As(&values)
	if err != nil {
		return tftypes.NewValue(tftypes.Object{}, tftypes.UnknownValue), fmt.Errorf("marshaling dynamic block to terraform value: %w", err)
	}

	val, err := encodeTfObjectDynamicPseudoType(d.OriginalValue, values)
	if err != nil {
		return tftypes.NewValue(tftypes.Object{}, tftypes.UnknownValue), fmt.Errorf("marshaling dynamic block to terraform value: %w", err)
	}

	return val, nil
}
