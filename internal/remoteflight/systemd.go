package remoteflight

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	xssh "golang.org/x/crypto/ssh"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
)

// SystemdUnit is a map structure representing any systemd unit. The first keys
// represent stanzas and the the values of each stanza is a map of filed names
// and values.
type SystemdUnit map[string]map[string]string

// Unitable is an interface for a type that can be converted into a systemd unit
type Unitable interface {
	ToUnit() (string, error)
}

var _ Unitable = (SystemdUnit)(nil)

// ToUnit converts a SystemdUnit to the textual representation.  Due to go maps
// not being ordered, the Unit may render differently each time that the function
// is called. In testing this hasn't shown any negative effects but might be
// confusing.
func (s SystemdUnit) ToUnit() (string, error) {
	unit := &strings.Builder{}

	for stanza, fields := range s {
		if len(fields) == 0 {
			continue
		}

		unit.WriteString(fmt.Sprintf("[%s]\n", stanza))
		for k, v := range fields {
			unit.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}

		unit.WriteString("\n")
	}

	return strings.TrimSpace(unit.String()), nil
}

// CreateSystemdUnitFileRequest is a systemd unit file creator
type CreateSystemdUnitFileRequest struct {
	SystemdUnit Unitable
	UnitPath    string
	Chmod       string
	Chown       string
}

// CreateSystemdUnitFileOpt is a functional option for an systemd unit request
type CreateSystemdUnitFileOpt func(*CreateSystemdUnitFileRequest) *CreateSystemdUnitFileRequest

// NewCreateSystemdUnitFileRequest takes functional options and returns a new
// systemd unit request
func NewCreateSystemdUnitFileRequest(opts ...CreateSystemdUnitFileOpt) *CreateSystemdUnitFileRequest {
	c := &CreateSystemdUnitFileRequest{}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithSystemdUnitUnitPath sets the unit name
func WithSystemdUnitUnitPath(path string) CreateSystemdUnitFileOpt {
	return func(u *CreateSystemdUnitFileRequest) *CreateSystemdUnitFileRequest {
		u.UnitPath = path
		return u
	}
}

// WithSystemdUnitFile sets systemd unit to use
func WithSystemdUnitFile(unit Unitable) CreateSystemdUnitFileOpt {
	return func(u *CreateSystemdUnitFileRequest) *CreateSystemdUnitFileRequest {
		u.SystemdUnit = unit
		return u
	}
}

// WithSystemdUnitChmod sets systemd unit permissions
func WithSystemdUnitChmod(chmod string) CreateSystemdUnitFileOpt {
	return func(u *CreateSystemdUnitFileRequest) *CreateSystemdUnitFileRequest {
		u.Chmod = chmod
		return u
	}
}

// WithSystemdUnitChown sets systemd unit ownership
func WithSystemdUnitChown(chown string) CreateSystemdUnitFileOpt {
	return func(u *CreateSystemdUnitFileRequest) *CreateSystemdUnitFileRequest {
		u.Chown = chown
		return u
	}
}

// CreateSystemdUnitFile takes a context, transport, and create request and
// creates the systemd unit file.
func CreateSystemdUnitFile(ctx context.Context, ssh it.Transport, req *CreateSystemdUnitFileRequest) error {
	unit, err := req.SystemdUnit.ToUnit()
	if err != nil {
		return fmt.Errorf("marshaling systemd unit: %w", err)
	}

	if req.UnitPath == "" {
		return fmt.Errorf("you must provide a unit destination path")
	}

	copyOpts := []CopyFileRequestOpt{
		WithCopyFileContent(tfile.NewReader(unit)),
		WithCopyFileDestination(req.UnitPath),
	}

	if req.Chmod != "" {
		copyOpts = append(copyOpts, WithCopyFileChmod(req.Chmod))
	}

	if req.Chown != "" {
		copyOpts = append(copyOpts, WithCopyFileChown(req.Chown))
	}

	return CopyFile(ctx, ssh, NewCopyFileRequest(copyOpts...))
}

// SystemctlCommandReq is a sysmtemctl command request
type SystemctlCommandReq struct {
	User       bool                // enable user mode
	Name       string              // unit name
	Type       SystemdUnitType     // unit type
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

// SystemdUnitType is the systemd unit type to operate on
type SystemdUnitType int

// SystemdUnitTypes are the system unit types
const (
	SystemdUnitTypeNotSet SystemdUnitType = iota
	SystemdUnitTypeService
	SystemdUnitTypeSocket
	SystemdUnitTypeDevice
	SystemdUnitTypeMount
	SystemdUnitTypeAutomount
	SystemdUnitTypeSwap
	SystemdUnitTypeTarget
	SystemdUnitTypePath
	SystemdUnitTypeTimer
	SystemdUnitTypeSlice
	SystemdUnitTypeScope
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
		Type: SystemdUnitTypeService,
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
func WithSystemctlCommandUnitType(typ SystemdUnitType) RunSystemctlCommandOpt {
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
	case SystemdUnitTypeNotSet:
	case SystemdUnitTypeService:
		if unitSuffix != ".service" {
			c.Name = fmt.Sprintf("%s.service", c.Name)
		}
	case SystemdUnitTypeSocket, SystemdUnitTypeDevice,
		SystemdUnitTypeMount, SystemdUnitTypeAutomount,
		SystemdUnitTypeSwap, SystemdUnitTypeTarget,
		SystemdUnitTypePath, SystemdUnitTypeTimer,
		SystemdUnitTypeSlice, SystemdUnitTypeScope:
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

// RunSystemctlCommand runs a systemctl command request
func RunSystemctlCommand(ctx context.Context, ssh it.Transport, req *SystemctlCommandReq) (*SystemctlCommandRes, error) {
	//_, stderr, err := ssh.Run(ctx, command.New("sudo systemctl --now enable vault"))
	res := &SystemctlCommandRes{}
	cmd, err := req.String()
	if err != nil {
		return res, err
	}

	res.Stdout, res.Stderr, err = ssh.Run(ctx, command.New(cmd, command.WithEnvVars(req.Env)))
	if err != nil {
		var exitError *xssh.ExitError
		if errors.As(err, &exitError) {
			res.Status = exitError.Waitmsg.ExitStatus()
		}
	}

	return res, err
}
