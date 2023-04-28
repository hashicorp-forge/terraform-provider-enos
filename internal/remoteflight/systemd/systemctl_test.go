package systemd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemctlCommandString(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		expected   string
		cmd        *SystemctlCommandReq
		shouldFail bool
	}{
		{
			"sudo systemctl start vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandStart,
			},
			false,
		},
		{
			"sudo systemctl --user start vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandStart,
				User:       true,
			},
			false,
		},
		{
			"sudo systemctl enable vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandEnable,
			},
			false,
		},
		{
			"sudo systemctl --now enable vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandEnable,
				Options:    "--now",
			},
			false,
		},
		{
			"sudo systemctl --user --now enable vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandEnable,
				Options:    "--now",
				User:       true,
			},
			false,
		},
		{
			"sudo systemctl is-active consul.service",
			&SystemctlCommandReq{
				Name:       "consul",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandIsActive,
			},
			false,
		},
		{
			"sudo systemctl stop vault.service",
			&SystemctlCommandReq{
				Name:       "vault.service",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandStop,
			},
			false,
		},
		{
			"sudo systemctl status vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandStatus,
			},
			false,
		},
		{
			"sudo systemctl status vault.service",
			&SystemctlCommandReq{
				Name:       "vault.service",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandStatus,
			},
			false,
		},
		{
			"sudo systemctl reload vault.service",
			&SystemctlCommandReq{
				Name:       "vault.service",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandReload,
			},
			false,
		},
		{
			"sudo systemctl restart vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandRestart,
			},
			false,
		},
		{
			"sudo systemctl kill vault.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandKill,
			},
			false,
		},
		{
			"sudo systemctl daemon-reload",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandDaemonReload,
			},
			false,
		},
		{
			"sudo systemctl kill vault.service another.service",
			&SystemctlCommandReq{
				Name:       "vault",
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandKill,
				Pattern:    "another.service",
			},
			false,
		},
		{
			"sudo systemctl --full --all --plain --no-legend list-units -t service another.service",
			&SystemctlCommandReq{
				Type:       UnitTypeService,
				SubCommand: SystemctlSubCommandListUnits,
				Options:    "--full --all --plain --no-legend",
				Pattern:    "another.service",
			},
			false,
		},
		{
			"sudo systemctl list-units -t mount",
			&SystemctlCommandReq{
				Type:       UnitTypeMount,
				SubCommand: SystemctlSubCommandListUnits,
			},
			false,
		},
	} {
		test := test
		t.Run(test.expected, func(t *testing.T) {
			t.Parallel()

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
