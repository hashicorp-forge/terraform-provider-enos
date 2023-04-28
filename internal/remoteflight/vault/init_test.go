package vault

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
			`/bin/vault operator init -format=json -key-shares=7 -key-threshold=5 -pgp-keys='keybase:ryan,keybase:jaymala,keybase:kier'`,
			[]InitRequestOpt{
				WithInitRequestBinPath("/bin/vault"),
				WithInitRequestVaultAddr("http://127.0.0.1:8200"),
				WithInitRequestKeyShares(7),
				WithInitRequestKeyThreshold(5),
				WithInitRequestPGPKeys([]string{"keybase:ryan", "keybase:jaymala", "keybase:kier"}),
			},
		},
		{
			`/bin/vault operator init -format=json -key-shares=7 -recovery-shares=7 -recovery-threshold=5 -recovery-pgp-keys='keybase:ryan,keybase:jaymala,keybase:kier' -root-token-pgp-key='keybase:hashicorp' -stored-shares=7`,
			[]InitRequestOpt{
				WithInitRequestBinPath("/bin/vault"),
				WithInitRequestVaultAddr("http://127.0.0.1:8200"),
				WithInitRequestStoredShares(7),
				WithInitRequestRecoveryShares(7),
				WithInitRequestRecoveryThreshold(5),
				WithInitRequestRecoveryPGPKeys([]string{"keybase:ryan", "keybase:jaymala", "keybase:kier"}),
				WithInitRequestRootTokenPGPKey("keybase:hashicorp"),
				WithInitRequestKeyShares(7),
			},
		},
		{
			`/bin/vault operator init -format=json -consul-auto=true -consul-service='vault'`,
			[]InitRequestOpt{
				WithInitRequestBinPath("/bin/vault"),
				WithInitRequestVaultAddr("http://127.0.0.1:8200"),
				WithInitRequestConsulAuto(true),
				WithInitRequestConsulService("vault"),
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
				WithInitRequestBinPath("/bin/vault"),
				WithInitRequestVaultAddr("http://127.0.0.1:8200"),
			},
			true,
		},
		{
			"missing vault addr",
			[]InitRequestOpt{
				WithInitRequestVaultAddr("http://127.0.0.1:8200"),
			},
			false,
		},
		{
			"missing bin path",
			[]InitRequestOpt{
				WithInitRequestVaultAddr("http://127.0.0.1:8200"),
			},
			false,
		},
		{
			"mismatched stored shares and key shares",
			[]InitRequestOpt{
				WithInitRequestBinPath("/bin/vault"),
				WithInitRequestVaultAddr("http://127.0.0.1:8200"),
				WithInitRequestStoredShares(5),
			},
			false,
		},
		{
			"has equal stored shares and key shares",
			[]InitRequestOpt{
				WithInitRequestBinPath("/bin/vault"),
				WithInitRequestVaultAddr("http://127.0.0.1:8200"),
				WithInitRequestStoredShares(5),
				WithInitRequestKeyShares(5),
			},
			true,
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
