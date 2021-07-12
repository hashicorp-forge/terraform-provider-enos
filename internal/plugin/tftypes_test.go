package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestTFBoolGetAndValue tests that tfBool type returns the correct values
func TestTFBoolGetAndValue(t *testing.T) {
	for _, test := range []struct {
		in    *tfBool
		value tftypes.Value
		val   bool
		ok    bool
	}{
		{
			&tfBool{unknown: true},
			tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue),
			false,
			false,
		},
		{
			&tfBool{null: true},
			tftypes.NewValue(tftypes.Bool, nil),
			false,
			false,
		},
		{
			&tfBool{val: true},
			tftypes.NewValue(tftypes.Bool, true),
			true,
			true,
		},
		{
			&tfBool{val: false},
			tftypes.NewValue(tftypes.Bool, false),
			false,
			true,
		},
	} {
		require.True(t, test.in.TFValue().Equal(test.value))
		val, ok := test.in.Get()
		require.Equal(t, test.val, val)
		require.Equal(t, test.ok, ok)
	}
}

// TestTFBoolSet tests that a tfBool returns the value that is set
func TestTFBoolSet(t *testing.T) {
	for _, b := range []bool{true, false} {
		tb := &tfBool{}
		tb.Set(b)

		val, ok := tb.Get()
		require.True(t, ok)
		require.Equal(t, b, tb.val)
		require.Equal(t, b, val)
	}
}

// TestTFNumGetAndValue tests that the tfNum returns the correct values
func TestTFNumGetAndValue(t *testing.T) {
	for _, test := range []struct {
		in    *tfNum
		value tftypes.Value
		val   int
		ok    bool
	}{
		{
			&tfNum{unknown: true, val: 3},
			tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
			3,
			false,
		},
		{
			&tfNum{null: true, val: 4},
			tftypes.NewValue(tftypes.Number, nil),
			4,
			false,
		},
		{
			&tfNum{val: 5},
			tftypes.NewValue(tftypes.Number, 5),
			5,
			true,
		},
	} {
		require.True(t, test.in.TFValue().Equal(test.value))
		val, ok := test.in.Get()
		require.Equal(t, test.val, val)
		require.Equal(t, test.ok, ok)
	}
}

// TestTFStringSlice tests that the tfStringSlice type returns the correct values
func TestTFStringSliceGetAndValue(t *testing.T) {
	for _, test := range []struct {
		in    *tfStringSlice
		value tftypes.Value
		val   []string
		ok    bool
	}{
		{
			&tfStringSlice{unknown: true, val: []string{"foo"}},
			tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, tftypes.UnknownValue),
			[]string{"foo"},
			false,
		},
		{
			&tfStringSlice{null: true, val: []string{"foo"}},
			tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			[]string{"foo"},
			false,
		},
		{
			&tfStringSlice{val: []string{"foo", "bar"}},
			tfMarshalStringSlice([]string{"foo", "bar"}),
			[]string{"foo", "bar"},
			true,
		},
		{
			&tfStringSlice{val: []string{"foo", UnknownString}},
			tfMarshalStringSlice([]string{"foo", UnknownString}),
			[]string{"foo", UnknownString},
			true,
		},
	} {
		require.True(t, test.in.TFValue().Equal(test.value))
		val, ok := test.in.Get()
		require.Equal(t, test.val, val)
		require.Equal(t, test.ok, ok)
	}
}
