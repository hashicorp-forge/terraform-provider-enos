package systemd

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SystemctlCommandReq is a sysmtemctl command request
type SystemctlCommandReq struct {
	User       bool                // enable user mode
	Name       string              // unit name
	Type       UnitType            // unit type
	SubCommand SystemctlSubCommand // systemctl command
	Pattern    string              // optional command pattern to pass as args
	Options    string              // optional command options to pass as args
	Env        map[string]string   // any environment variables that need to be set
}

// SystemctlCommandRes is the command response
type SystemctlCommandRes struct {
	Stdout string
	Stderr string
	Status int
}

// UnitType is the systemd unit type to operate on
type UnitType int

// SystemdUnitTypes are the system unit types
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

// SystemctlSubCommand is the systemctl sub command to use
type SystemctlSubCommand int

// SystemctlSubCommands are the systemctl sub commands
const (
	SystemctlSubCommandNotSet SystemctlSubCommand = iota
	SystemctlSubCommandStatus
	SystemctlSubCommandStart
	SystemctlSubCommandEnable
	SystemctlSubCommandStop
	SystemctlSubCommandReload
	SystemctlSubCommandRestart
	SystemctlSubCommandKill
	SystemctlSubCommandDaemonReload
)

// RunSystemctlCommandOpt is a functional option for an systemd unit request
type RunSystemctlCommandOpt func(*SystemctlCommandReq) *SystemctlCommandReq

// NewRunSystemctlCommand takes functional options and returns a new
// systemd command
func NewRunSystemctlCommand(opts ...RunSystemctlCommandOpt) *SystemctlCommandReq {
	c := &SystemctlCommandReq{
		Type: UnitTypeService,
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithSystemctlCommandUser sets command to --user mode
func WithSystemctlCommandUser() RunSystemctlCommandOpt {
	return func(c *SystemctlCommandReq) *SystemctlCommandReq {
		c.User = true
		return c
	}
}

// WithSystemctlCommandUnitName sets the command unit name
func WithSystemctlCommandUnitName(unit string) RunSystemctlCommandOpt {
	return func(c *SystemctlCommandReq) *SystemctlCommandReq {
		c.Name = unit
		return c
	}
}

// WithSystemctlCommandUnitType sets the command unit type
func WithSystemctlCommandUnitType(typ UnitType) RunSystemctlCommandOpt {
	return func(c *SystemctlCommandReq) *SystemctlCommandReq {
		c.Type = typ
		return c
	}
}

// WithSystemctlCommandSubCommand sets the systemctl sub-command
func WithSystemctlCommandSubCommand(cmd SystemctlSubCommand) RunSystemctlCommandOpt {
	return func(c *SystemctlCommandReq) *SystemctlCommandReq {
		c.SubCommand = cmd
		return c
	}
}

// WithSystemctlCommandPattern sets any optional pattern to pass in
func WithSystemctlCommandPattern(pattern string) RunSystemctlCommandOpt {
	return func(c *SystemctlCommandReq) *SystemctlCommandReq {
		c.Pattern = pattern
		return c
	}
}

// WithSystemctlCommandOptions sets any optional options to pass in
func WithSystemctlCommandOptions(options string) RunSystemctlCommandOpt {
	return func(c *SystemctlCommandReq) *SystemctlCommandReq {
		c.Options = options
		return c
	}
}

// String returns the command request as a systemctl string
func (c *SystemctlCommandReq) String() (string, error) {
	cmd := &strings.Builder{}
	cmd.WriteString("sudo systemctl ")

	if c.User {
		cmd.WriteString("--user ")
	}

	if c.Options != "" {
		cmd.WriteString(fmt.Sprintf("%s ", c.Options))
	}

	unitSuffix := filepath.Ext(c.Name)
	switch c.Type {
	case UnitTypeNotSet:
	case UnitTypeService:
		if unitSuffix != ".service" {
			c.Name = fmt.Sprintf("%s.service", c.Name)
		}
	case UnitTypeSocket, UnitTypeDevice,
		UnitTypeMount, UnitTypeAutomount,
		UnitTypeSwap, UnitTypeTarget,
		UnitTypePath, UnitTypeTimer,
		UnitTypeSlice, UnitTypeScope:
		return "", fmt.Errorf("support for unit type has not been implemented yet")
	default:
		return "", fmt.Errorf("unknown unit type: %d", c.Type)
	}

	switch c.SubCommand {
	case SystemctlSubCommandStatus:
		cmd.WriteString("status")
		if c.Name != "" {
			cmd.WriteString(fmt.Sprintf(" %s", c.Name))
		}
	case SystemctlSubCommandEnable:
		cmd.WriteString(fmt.Sprintf("enable %s", c.Name))
	case SystemctlSubCommandStart:
		cmd.WriteString(fmt.Sprintf("start %s", c.Name))
	case SystemctlSubCommandStop:
		cmd.WriteString(fmt.Sprintf("stop %s", c.Name))
	case SystemctlSubCommandReload:
		cmd.WriteString(fmt.Sprintf("reload %s", c.Name))
	case SystemctlSubCommandRestart:
		cmd.WriteString(fmt.Sprintf("restart %s", c.Name))
	case SystemctlSubCommandKill:
		cmd.WriteString(fmt.Sprintf("kill %s", c.Name))
	case SystemctlSubCommandDaemonReload:
		cmd.WriteString("daemon-reload")
	default:
		return "", fmt.Errorf("unknown command: %d", c.SubCommand)
	}
	if c.Pattern != "" {
		cmd.WriteString(fmt.Sprintf(" %s", c.Pattern))
	}

	return cmd.String(), nil
}
