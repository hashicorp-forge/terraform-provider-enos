package systemd

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SystemctlCommandReq is a sysmtemctl command request.
type SystemctlCommandReq struct {
	User       bool                // enable user mode
	Name       string              // unit name
	Type       UnitType            // unit type
	SubCommand SystemctlSubCommand // systemctl command
	Pattern    string              // optional command pattern to pass as args
	Options    string              // optional command options to pass as args
	Env        map[string]string   // any environment variables that need to be set
}

// SystemctlCommandRes is the command response.
type SystemctlCommandRes struct {
	Stdout string
	Stderr string
	Status SystemctlStatusCode
}

// SystemctlSubCommand is the systemctl sub command to use.
type SystemctlSubCommand int

// SystemctlSubCommands are the systemctl sub commands.
const (
	SystemctlSubCommandNotSet SystemctlSubCommand = iota
	SystemctlSubCommandDaemonReload
	SystemctlSubCommandEnable
	SystemctlSubCommandIsActive
	SystemctlSubCommandKill
	SystemctlSubCommandListUnits
	SystemctlSubCommandReload
	SystemctlSubCommandRestart
	SystemctlSubCommandShow
	SystemctlSubCommandStart
	SystemctlSubCommandStatus
	SystemctlSubCommandStop
)

// SystemctlStatusCode is a systemctl exit code. Systemctl attempts to use LSB
// exit codes, however, there are lots of corner cases which make certain codes
// unreliable and not all sub-commands adhere to these codes. As such, we
// should strive to interpret a units state by looking at its properties
// whenever possible
//
// Further reading:
//   - https://www.freedesktop.org/software/systemd/man/systemctl.html#Exit%20status
//   - https://freedesktop.org/software/systemd/man/systemd.exec.html#Process%20Exit%20Codes
//   - https://bugs.freedesktop.org/show_bug.cgi?id=77507
type SystemctlStatusCode int

const (
	StatusOK         SystemctlStatusCode = 0
	StatusNotFailed  SystemctlStatusCode = 1
	StatusNotActive  SystemctlStatusCode = 3
	StatusNoSuchUnit SystemctlStatusCode = 4
	StatusUnknown    SystemctlStatusCode = 9
)

// RunSystemctlCommandOpt is a functional option for an systemd unit request.
type RunSystemctlCommandOpt func(*SystemctlCommandReq) *SystemctlCommandReq

// NewRunSystemctlCommand takes functional options and returns a new
// systemd command.
func NewRunSystemctlCommand(opts ...RunSystemctlCommandOpt) *SystemctlCommandReq {
	c := &SystemctlCommandReq{
		Type: UnitTypeService,
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithSystemctlCommandUser sets command to --user mode.
func WithSystemctlCommandUser() RunSystemctlCommandOpt {
	return func(c *SystemctlCommandReq) *SystemctlCommandReq {
		c.User = true
		return c
	}
}

// WithSystemctlCommandUnitName sets the command unit name.
func WithSystemctlCommandUnitName(unit string) RunSystemctlCommandOpt {
	return func(c *SystemctlCommandReq) *SystemctlCommandReq {
		c.Name = unit
		return c
	}
}

// WithSystemctlCommandUnitType sets the command unit type.
func WithSystemctlCommandUnitType(typ UnitType) RunSystemctlCommandOpt {
	return func(c *SystemctlCommandReq) *SystemctlCommandReq {
		c.Type = typ
		return c
	}
}

// WithSystemctlCommandSubCommand sets the systemctl sub-command.
func WithSystemctlCommandSubCommand(cmd SystemctlSubCommand) RunSystemctlCommandOpt {
	return func(c *SystemctlCommandReq) *SystemctlCommandReq {
		c.SubCommand = cmd
		return c
	}
}

// WithSystemctlCommandPattern sets any optional pattern to pass in.
func WithSystemctlCommandPattern(pattern string) RunSystemctlCommandOpt {
	return func(c *SystemctlCommandReq) *SystemctlCommandReq {
		c.Pattern = pattern
		return c
	}
}

// WithSystemctlCommandOptions sets any optional options to pass in.
func WithSystemctlCommandOptions(options string) RunSystemctlCommandOpt {
	return func(c *SystemctlCommandReq) *SystemctlCommandReq {
		c.Options = options
		return c
	}
}

func (s SystemctlStatusCode) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusNotFailed:
		return "not-failed"
	case StatusNotActive:
		return "not-active"
	case StatusNoSuchUnit:
		return "no-such-unit"
	case StatusUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// String returns the command request as a systemctl string.
func (c *SystemctlCommandReq) String() (string, error) {
	cmd := &strings.Builder{}
	cmd.WriteString("sudo systemctl ")

	if c.User {
		cmd.WriteString("--user ")
	}

	if c.Options != "" {
		cmd.WriteString(fmt.Sprintf("%s ", c.Options))
	}

	unitName := c.Name
	unitNameSuffix := filepath.Ext(unitName)
	unitTypeSuffix := fmt.Sprintf(".%s", c.Type)
	if unitNameSuffix != unitTypeSuffix {
		unitName = fmt.Sprintf("%s%s", unitName, unitTypeSuffix)
	}

	switch c.SubCommand {
	case SystemctlSubCommandNotSet:
		return "", fmt.Errorf("sub command is not set")
	case SystemctlSubCommandDaemonReload:
		cmd.WriteString("daemon-reload")
	case SystemctlSubCommandIsActive:
		cmd.WriteString(fmt.Sprintf("is-active %s", unitName))
	case SystemctlSubCommandEnable:
		cmd.WriteString(fmt.Sprintf("enable %s", unitName))
	case SystemctlSubCommandKill:
		cmd.WriteString(fmt.Sprintf("kill %s", unitName))
	case SystemctlSubCommandListUnits:
		cmd.WriteString(fmt.Sprintf("list-units -t %s", c.Type))
	case SystemctlSubCommandReload:
		cmd.WriteString(fmt.Sprintf("reload %s", unitName))
	case SystemctlSubCommandRestart:
		cmd.WriteString(fmt.Sprintf("restart %s", unitName))
	case SystemctlSubCommandShow:
		cmd.WriteString(fmt.Sprintf("show %s", unitName))
	case SystemctlSubCommandStart:
		cmd.WriteString(fmt.Sprintf("start %s", unitName))
	case SystemctlSubCommandStatus:
		cmd.WriteString("status")
		if unitName != "" {
			cmd.WriteString(fmt.Sprintf(" %s", unitName))
		}
	case SystemctlSubCommandStop:
		cmd.WriteString(fmt.Sprintf("stop %s", unitName))
	default:
		return "", fmt.Errorf("unknown command: %d", c.SubCommand)
	}
	if c.Pattern != "" {
		cmd.WriteString(fmt.Sprintf(" %s", c.Pattern))
	}

	return cmd.String(), nil
}
