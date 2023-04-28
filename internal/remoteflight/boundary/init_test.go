package boundary

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestInitRequestString tests that the init request becomes a valid string.
func TestInitRequestString(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		expected string
		opts     []InitRequestOpt
	}{
		{
			`/bin/boundary database init -format json -config=/etc/boundary/boundary.hcl`,
			[]InitRequestOpt{
				WithInitRequestBinName("boundary"),
				WithInitRequestBinPath("/bin"),
				WithInitRequestConfigPath("/etc/boundary"),
			},
		},
	} {
		test := test
		t.Run(test.expected, func(t *testing.T) {
			t.Parallel()
			req := NewInitRequest(test.opts...)
			require.NoError(t, req.Validate())
			require.Equal(t, test.expected, req.String())
		})
	}
}

// TestInitRequestValidate tests the init request validation.
func TestInitRequestValidate(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		desc string
		opts []InitRequestOpt
		pass bool
	}{
		{
			"has required args",
			[]InitRequestOpt{
				WithInitRequestBinPath("/bin"),
				WithInitRequestConfigPath("/etc/boundary"),
			},
			true,
		},
		{
			"missing config path",
			[]InitRequestOpt{
				WithInitRequestBinName("boundary-test"),
				WithInitRequestBinPath("/opt/boundary"),
			},
			false,
		},
		{
			"missing bin path",
			[]InitRequestOpt{
				WithInitRequestBinName("boundary-test"),
				WithInitRequestConfigPath("/etc/boundary"),
			},
			false,
		},
	} {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			req := NewInitRequest(test.opts...)
			if test.pass {
				require.NoError(t, req.Validate())
			} else {
				require.Error(t, req.Validate())
			}
		})
	}
}
