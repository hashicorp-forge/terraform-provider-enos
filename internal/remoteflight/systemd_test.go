package remoteflight

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemctlCommandString(t *testing.T) {
	for _, test := range []struct {
		expected   string
		cmd        *SystemctlCommandReq
		shouldFail bool
	}{
		{
			"sudo systemctl start vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       SystemdUnitTypeService,
				SubCommand: SystemctlSubCommandStart,
			},
			false,
		},
		{
			"sudo systemctl --user start vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       SystemdUnitTypeService,
				SubCommand: SystemctlSubCommandStart,
				User:       true,
			},
			false,
		},
		{
			"sudo systemctl enable vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       SystemdUnitTypeService,
				SubCommand: SystemctlSubCommandEnable,
			},
			false,
		},
		{
			"sudo systemctl --now enable vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       SystemdUnitTypeService,
				SubCommand: SystemctlSubCommandEnable,
				Options:    "--now",
			},
			false,
		},
		{
			"sudo systemctl --user --now enable vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       SystemdUnitTypeService,
				SubCommand: SystemctlSubCommandEnable,
				Options:    "--now",
				User:       true,
			},
			false,
		},
		{
			"sudo systemctl stop vault.service",
			&SystemctlCommandReq{
				Name:       "vault.service",
				Type:       SystemdUnitTypeService,
				SubCommand: SystemctlSubCommandStop,
			},
			false,
		},
		{
			"sudo systemctl status vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       SystemdUnitTypeService,
				SubCommand: SystemctlSubCommandStatus,
			},
			false,
		},
		{
			"sudo systemctl status vault.service",
			&SystemctlCommandReq{
				Name:       "vault.service",
				Type:       SystemdUnitTypeService,
				SubCommand: SystemctlSubCommandStatus,
			},
			false,
		},
		{
			"sudo systemctl reload vault.service",
			&SystemctlCommandReq{
				Name:       "vault.service",
				Type:       SystemdUnitTypeService,
				SubCommand: SystemctlSubCommandReload,
			},
			false,
		},
		{
			"sudo systemctl restart vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       SystemdUnitTypeService,
				SubCommand: SystemctlSubCommandRestart,
			},
			false,
		},
		{
			"sudo systemctl kill vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       SystemdUnitTypeService,
				SubCommand: SystemctlSubCommandKill,
			},
			false,
		},
		{
			"sudo systemctl daemon-reload",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       SystemdUnitTypeService,
				SubCommand: SystemctlSubCommandDaemonReload,
			},
			false,
		},
		{
			"sudo systemctl kill vault.service another.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       SystemdUnitTypeService,
				SubCommand: SystemctlSubCommandKill,
				Pattern:    "another.service",
			},
			false,
		},
	} {
		t.Run(test.expected, func(t *testing.T) {
			out, err := test.cmd.String()
			if test.shouldFail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expected, out)
			}
		})
	}
}
