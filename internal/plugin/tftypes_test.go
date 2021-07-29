package plugin

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestTFBoolGetAndValue tests that tfBool type returns the correct values
func TestTFBoolGetAndValue(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		desc  string
		in    *tfBool
		value tftypes.Value
		val   bool
		ok    bool
	}{
		{
			"unknown",
			&tfBool{Unknown: true},
			tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue),
			false,
			false,
		},
		{
			"null",
			&tfBool{Null: true},
			tftypes.NewValue(tftypes.Bool, nil),
			false,
			false,
		},
		{
			"true",
			&tfBool{Val: true},
			tftypes.NewValue(tftypes.Bool, true),
			true,
			true,
		},
		{
			"false",
			&tfBool{Val: false},
			tftypes.NewValue(tftypes.Bool, false),
			false,
			true,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			require.True(t, test.in.TFValue().Equal(test.value))
			val, ok := test.in.Get()
			require.Equal(t, test.val, val)
			require.Equal(t, test.ok, ok)
		})
	}
}

// TestTFBoolSet tests that a tfBool returns the value that is set
func TestTFBoolSet(t *testing.T) {
	for _, b := range []bool{true, false} {
		tb := &tfBool{}
		tb.Set(b)

		val, ok := tb.Get()
		require.True(t, ok)
		require.Equal(t, b, tb.Val)
		require.Equal(t, b, val)
	}
}

// TestTFNumGetAndValue tests that the tfNum returns the correct values
func TestTFNumGetAndValue(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		desc  string
		in    *tfNum
		value tftypes.Value
		val   int
		ok    bool
	}{
		{
			"unknown",
			&tfNum{Unknown: true, Val: 3},
			tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
			3,
			false,
		},
		{
			"null",
			&tfNum{Null: true, Val: 4},
			tftypes.NewValue(tftypes.Number, nil),
			4,
			false,
		},
		{
			"known",
			&tfNum{Val: 5},
			tftypes.NewValue(tftypes.Number, 5),
			5,
			true,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			require.True(t, test.in.TFValue().Equal(test.value))
			val, ok := test.in.Get()
			require.Equal(t, test.val, val)
			require.Equal(t, test.ok, ok)
		})
	}
}

// TestTFStringGetAndValue tests that the tfString returns the correct values
func TestTFStringGetAndValue(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		desc  string
		in    *tfString
		value tftypes.Value
		val   string
		ok    bool
	}{
		{
			"unknown",
			&tfString{Unknown: true, Val: "1"},
			tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			"1",
			false,
		},
		{
			"null",
			&tfString{Null: true, Val: "2"},
			tftypes.NewValue(tftypes.String, nil),
			"2",
			false,
		},
		{
			"known",
			&tfString{Val: "5"},
			tftypes.NewValue(tftypes.String, "5"),
			"5",
			true,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			require.True(t, test.in.TFValue().Equal(test.value))
			val, ok := test.in.Get()
			require.Equal(t, test.val, val)
			require.Equal(t, test.ok, ok)
		})
	}
}

// TestTFStringSliceGetAndValue tests that the tfStringSlice type returns the correct values
func TestTFStringSliceGetAndValue(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		desc  string
		in    *tfStringSlice
		value tftypes.Value
		val   []string
		ok    bool
	}{
		{
			"unknown",
			&tfStringSlice{Unknown: true, Val: []*tfString{{Val: "foo"}}},
			tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, tftypes.UnknownValue),
			[]string{},
			false,
		},
		{
			"null",
			&tfStringSlice{Null: true, Val: []*tfString{{Val: "foo"}}},
			tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			[]string{},
			false,
		},
		{
			"fully known",
			&tfStringSlice{Val: []*tfString{{Val: "foo"}, {Val: "bar"}}},
			tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{
				tftypes.NewValue(tftypes.String, "foo"),
				tftypes.NewValue(tftypes.String, "bar"),
			}),
			[]string{"foo", "bar"},
			true,
		},
		{
			"known with unknown child",
			&tfStringSlice{Val: []*tfString{{Unknown: true}}},
			tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{
				tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			}),
			[]string{},
			false,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			require.True(t, test.in.TFValue().Equal(test.value))
			val, ok := test.in.GetStrings()
			require.Equal(t, test.val, val)
			require.Equal(t, test.ok, ok)
		})
	}
}

// TestTFStringMapGetAndValue tests that the tfStringMap type returns the correct values
func TestTFStringMapGetAndValue(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		desc  string
		in    *tfStringMap
		value tftypes.Value
		val   map[string]string
		ok    bool
	}{
		{
			"unknown",
			&tfStringMap{Unknown: true, Val: map[string]*tfString{"foo": {Val: "foo"}}},
			tftypes.NewValue(tftypes.Map{AttributeType: tftypes.String}, tftypes.UnknownValue),
			map[string]string{},
			false,
		},
		{
			"null",
			&tfStringMap{Null: true, Val: map[string]*tfString{"foo": {Val: "foo"}}},
			tftypes.NewValue(tftypes.Map{AttributeType: tftypes.String}, nil),
			map[string]string{},
			false,
		},
		{
			"fully known",
			&tfStringMap{Val: map[string]*tfString{"foo": {Val: "foo"}, "bar": {Val: "bar"}}},
			tftypes.NewValue(tftypes.Map{AttributeType: tftypes.String}, map[string]tftypes.Value{
				"foo": tftypes.NewValue(tftypes.String, "foo"),
				"bar": tftypes.NewValue(tftypes.String, "bar"),
			}),
			map[string]string{"foo": "foo", "bar": "bar"},
			true,
		},
		{
			"known with unknown child",
			&tfStringMap{Val: map[string]*tfString{"foo": {Unknown: true}}},
			tftypes.NewValue(tftypes.Map{AttributeType: tftypes.String}, map[string]tftypes.Value{
				"foo": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
			}),
			map[string]string{},
			false,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			require.True(t, test.in.TFValue().Equal(test.value))
			val, ok := test.in.GetStrings()
			require.Equal(t, test.val, val)
			require.Equal(t, test.ok, ok)
		})
	}
}

// TestTFObjectGetAndValue tests that the tfObject type returns the correct values
func TestTFObjectGetAndValue(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		desc  string
		in    func() *tfObject
		value tftypes.Value
		val   map[string]interface{}
		ok    bool
	}{
		{
			"unknown",
			func() *tfObject {
				obj := newTfObject()
				obj.Set(map[string]interface{}{"foo": &tfString{Val: "foo"}, "bar": &tfBool{Val: false}})
				obj.Unknown = true
				return obj
			},
			tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
					"bar": tftypes.Bool,
				},
			}, tftypes.UnknownValue),
			map[string]interface{}{},
			false,
		},
		{
			"null",
			func() *tfObject {
				obj := newTfObject()
				obj.Set(map[string]interface{}{"foo": &tfString{Val: "foo"}, "bar": &tfBool{Val: false}})
				obj.Null = true
				return obj
			},
			tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
					"bar": tftypes.Bool,
				},
			}, nil),
			map[string]interface{}{},
			false,
		},
		{
			"fully known",
			func() *tfObject {
				obj := newTfObject()
				obj.Set(map[string]interface{}{"foo": &tfString{Val: "foo"}, "bar": &tfBool{Val: false}})
				return obj
			},
			tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
					"bar": tftypes.Bool,
				},
			}, map[string]tftypes.Value{
				"foo": tftypes.NewValue(tftypes.String, "foo"),
				"bar": tftypes.NewValue(tftypes.Bool, false),
			}),
			map[string]interface{}{"foo": "foo", "bar": false},
			true,
		},
		{
			"known with unknown child",
			func() *tfObject {
				obj := newTfObject()
				obj.Set(map[string]interface{}{"foo": &tfString{Unknown: true}, "bar": &tfBool{Val: false}})
				return obj
			},
			tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
					"bar": tftypes.Bool,
				},
			}, map[string]tftypes.Value{
				"foo": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
				"bar": tftypes.NewValue(tftypes.Bool, false),
			}),
			map[string]interface{}{},
			false,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			obj := test.in()
			require.True(t, obj.TFValue().Equal(test.value))
			val, ok := obj.GetObject()
			require.Equal(t, test.val, val)
			require.Equal(t, test.ok, ok)
		})
	}
}

// TestTFObjectSliceGetAndValue tests that the tfObjectSlice type returns the correct values
func TestTFObjectSliceGetAndValue(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		desc  string
		in    func() *tfObjectSlice
		value tftypes.Value
		val   []map[string]interface{}
		ok    bool
	}{
		{
			"unknown",
			func() *tfObjectSlice {
				obj := newTfObject()
				obj.Set(map[string]interface{}{"foo": &tfString{Val: "foo"}, "bar": &tfBool{Val: false}})
				objs := newTfObjectSlice()
				objs.AttrTypes = map[string]tftypes.Type{"foo": tftypes.String, "bar": tftypes.Bool}
				objs.Set([]*tfObject{obj})
				objs.Unknown = true
				return objs
			},
			tftypes.NewValue(tftypes.List{ElementType: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
					"bar": tftypes.Bool,
				},
			}}, tftypes.UnknownValue),
			[]map[string]interface{}{},
			false,
		},
		{
			"null",
			func() *tfObjectSlice {
				obj := newTfObject()
				obj.Set(map[string]interface{}{"foo": &tfString{Val: "foo"}, "bar": &tfBool{Val: false}})
				objs := newTfObjectSlice()
				objs.AttrTypes = map[string]tftypes.Type{"foo": tftypes.String, "bar": tftypes.Bool}
				objs.Set([]*tfObject{obj})
				objs.Null = true
				return objs
			},
			tftypes.NewValue(tftypes.List{ElementType: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
					"bar": tftypes.Bool,
				},
			}}, nil),
			[]map[string]interface{}{},
			false,
		},
		{
			"fully known",
			func() *tfObjectSlice {
				obj := newTfObject()
				obj.Set(map[string]interface{}{"foo": &tfString{Val: "foo"}, "bar": &tfBool{Val: false}})
				objs := newTfObjectSlice()
				objs.AttrTypes = map[string]tftypes.Type{"foo": tftypes.String, "bar": tftypes.Bool}
				objs.Set([]*tfObject{obj})
				return objs
			},
			tftypes.NewValue(tftypes.List{ElementType: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
					"bar": tftypes.Bool,
				},
			}}, []tftypes.Value{tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
					"bar": tftypes.Bool,
				},
			}, map[string]tftypes.Value{
				"foo": tftypes.NewValue(tftypes.String, "foo"),
				"bar": tftypes.NewValue(tftypes.Bool, false),
			})}),
			[]map[string]interface{}{{"foo": "foo", "bar": false}},
			true,
		},
		{
			"known with unknown child",
			func() *tfObjectSlice {
				obj := newTfObject()
				obj.Set(map[string]interface{}{"foo": &tfString{Unknown: true}, "bar": &tfBool{Val: false}})
				objs := newTfObjectSlice()
				objs.AttrTypes = map[string]tftypes.Type{"foo": tftypes.String, "bar": tftypes.Bool}
				objs.Set([]*tfObject{obj})
				return objs
			},
			tftypes.NewValue(tftypes.List{ElementType: tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
					"bar": tftypes.Bool,
				},
			}}, []tftypes.Value{tftypes.NewValue(tftypes.Object{
				AttributeTypes: map[string]tftypes.Type{
					"foo": tftypes.String,
					"bar": tftypes.Bool,
				},
			}, map[string]tftypes.Value{
				"foo": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
				"bar": tftypes.NewValue(tftypes.Bool, false),
			})}),
			[]map[string]interface{}{},
			false,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			objs := test.in()
			if !objs.TFValue().Equal(test.value) {
				t.Log(spew.Sdump(objs.TFValue()))
				t.Log(spew.Sdump(test.value))
				t.FailNow()
			}
			require.True(t, objs.TFValue().Equal(test.value))
			val, ok := objs.GetObjects()
			require.Equal(t, test.val, val)
			require.Equal(t, test.ok, ok)
		})
	}
}
