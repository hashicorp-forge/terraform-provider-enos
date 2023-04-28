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
	Status int
}

// UnitType is the systemd unit type to operate on.
type UnitType int

// SystemdUnitTypes are the system unit types.
const (
	UnitTypeNotSet UnitType = iota
	UnitTypeService
	UnitTypeSocket
	UnitTypeDevice
	UnitTypeMount
	UnitTypeAutomount
	UnitTypeSwap
	UnitTypeTarget
	UnitTypePath
	UnitTypeTimer
	UnitTypeSlice
	UnitTypeScope
)

func (u UnitType) String() string {
	switch u {
	case UnitTypeService, UnitTypeNotSet:
		return "service"
	case UnitTypeSocket:
		return "socket"
	case UnitTypeDevice:
		return "device"
	case UnitTypeMount:
		return "mount"
	case UnitTypeAutomount:
		return "automount"
	case UnitTypeSwap:
		return "swap"
	case UnitTypeTarget:
		return "target"
	case UnitTypePath:
		return "path"
	case UnitTypeTimer:
		return "timer"
	case UnitTypeSlice:
		return "slice"
	case UnitTypeScope:
		return "scope"
	default:
		return "service"
	}
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
	SystemctlSubCommandStart
	SystemctlSubCommandStatus
	SystemctlSubCommandStop
	SystemctlSubCommandReload
	SystemctlSubCommandRestart
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
	case SystemctlSubCommandStatus:
		cmd.WriteString("status")
		if unitName != "" {
			cmd.WriteString(fmt.Sprintf(" %s", unitName))
		}
	case SystemctlSubCommandEnable:
		cmd.WriteString(fmt.Sprintf("enable %s", unitName))
	case SystemctlSubCommandIsActive:
		cmd.WriteString(fmt.Sprintf("is-active %s", unitName))
	case SystemctlSubCommandStart:
		cmd.WriteString(fmt.Sprintf("start %s", unitName))
	case SystemctlSubCommandStop:
		cmd.WriteString(fmt.Sprintf("stop %s", unitName))
	case SystemctlSubCommandReload:
		cmd.WriteString(fmt.Sprintf("reload %s", unitName))
	case SystemctlSubCommandRestart:
		cmd.WriteString(fmt.Sprintf("restart %s", unitName))
	case SystemctlSubCommandKill:
		cmd.WriteString(fmt.Sprintf("kill %s", unitName))
	case SystemctlSubCommandListUnits:
		cmd.WriteString(fmt.Sprintf("list-units -t %s", c.Type))
	case SystemctlSubCommandDaemonReload:
		cmd.WriteString("daemon-reload")
	case SystemctlSubCommandNotSet:
		return "", fmt.Errorf("sub command is not set")
	default:
		return "", fmt.Errorf("unknown command: %d", c.SubCommand)
	}
	if c.Pattern != "" {
		cmd.WriteString(fmt.Sprintf(" %s", c.Pattern))
	}

	return cmd.String(), nil
}
