package systemd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/enos-provider/internal/log"
	"github.com/hashicorp/enos-provider/internal/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
	"github.com/hashicorp/enos-provider/internal/transport/file"
)

// StatusCode is a systemd exit code from a status check.
type StatusCode int

const (
	StatusActive   StatusCode = 0
	StatusInactive StatusCode = 3 // or, unfortunately, "activating", thanks systemd
	StatusUnknown  StatusCode = 9
)

// serviceInfoRegex is the regex used to parse the systemctl services output into a slice of ServiceInfo.
var serviceInfoRegex = regexp.MustCompile(`^(?P<unit>\S+)\.service\s+(?P<load>\S+)\s+(?P<active>\S+)\s+(?P<sub>\S+)\s+(?P<description>\S.*)$`)

// KnownServices are list of known HashiCorp services.
var KnownServices = []string{"boundary", "consul", "vault"}

type GetLogsRequest struct {
	Unit string
	Host string
}

type GetLogsResponse struct {
	Unit string
	Host string
	Logs []byte
}

// ServiceInfo is a list units of type service from systemctl command reference https://man7.org/linux/man-pages/man1/systemctl.1.html#COMMANDS
type ServiceInfo struct {
	Unit        string
	Load        string
	Active      string
	Sub         string
	Description string
}

var _ remoteflight.GetLogsResponse = (*GetLogsResponse)(nil)

// GetAppName implements remoteflight.GetLogsResponse.
func (s GetLogsResponse) GetAppName() string {
	return s.Unit
}

func (s GetLogsResponse) GetLogFileName() string {
	return fmt.Sprintf("%s_%s.log", s.Unit, s.Host)
}

func (s GetLogsResponse) GetLogs() []byte {
	return s.Logs
}

// Client an interface for a sysetmd client.
type Client interface {
	// ListServices gets the list of systemd services installed
	ListServices(ctx context.Context) ([]ServiceInfo, error)
	// GetLogs gets the logs for a process using journalctl
	GetLogs(ctx context.Context, req GetLogsRequest) (remoteflight.GetLogsResponse, error)
	// CreateUnitFile creates a systemd unit file for the provided request
	CreateUnitFile(ctx context.Context, req *CreateUnitFileRequest) error
	// RunSystemctlCommand runs a systemctl command for the provided request
	RunSystemctlCommand(ctx context.Context, req *SystemctlCommandReq) (*SystemctlCommandRes, error)
	// EnableService enables a systemd service with the provided unit name
	EnableService(ctx context.Context, unit string) error
	// StartService starts a systemd service with the provided unit name
	StartService(ctx context.Context, unit string) error
	// StopService stops a systemd service with the provided unit name
	StopService(ctx context.Context, unit string) error
	// RestartService restarts a systemd service with the provided unit name
	RestartService(ctx context.Context, unit string) error
	// ServiceStatus gets the service status for a systemd service with the provided unit name
	ServiceStatus(ctx context.Context, unit string) StatusCode
	// IsActiveService checks if the systemd service with the provided unit name is active (running)
	IsActiveService(ctx context.Context, unit string) bool
}

type client struct {
	transport it.Transport
	logger    log.Logger
}

var _ Client = (*client)(nil)

func NewClient(transport it.Transport, logger log.Logger) Client {
	return &client{
		transport: transport,
		logger:    logger,
	}
}

func (c *client) GetLogs(ctx context.Context, req GetLogsRequest) (remoteflight.GetLogsResponse, error) {
	stdout, stderr, err := c.transport.Run(ctx, command.New(fmt.Sprintf("journalctl -x -u %s", req.Unit)))
	if err != nil {
		return nil, fmt.Errorf("failed to get systemd logs, due to: %w", err)
	}

	if stderr != "" {
		c.logger.With("stderr", stderr).Error("stderr retrieving systemd logs")
	}

	if stdout == "" {
		c.logger.Debug("no systemd logs")
	}

	return GetLogsResponse{
		Unit: req.Unit,
		Host: req.Host,
		Logs: []byte(stdout),
	}, nil
}

// CreateUnitFile takes a context, transport, and create request and creates the systemd unit file.
func (c *client) CreateUnitFile(ctx context.Context, req *CreateUnitFileRequest) error {
	unit, err := req.Unit.ToIni()
	if err != nil {
		return fmt.Errorf("marshaling systemd unit: %w", err)
	}

	if req.UnitPath == "" {
		return fmt.Errorf("you must provide a unit destination path")
	}

	copyOpts := []remoteflight.CopyFileRequestOpt{
		remoteflight.WithCopyFileContent(file.NewReader(unit)),
		remoteflight.WithCopyFileDestination(req.UnitPath),
	}

	if req.Chmod != "" {
		copyOpts = append(copyOpts, remoteflight.WithCopyFileChmod(req.Chmod))
	}

	if req.Chown != "" {
		copyOpts = append(copyOpts, remoteflight.WithCopyFileChown(req.Chown))
	}

	return remoteflight.CopyFile(ctx, c.transport, remoteflight.NewCopyFileRequest(copyOpts...))
}

// ListServices gets the list of systemd services installed.
func (c *client) ListServices(ctx context.Context) ([]ServiceInfo, error) {
	res, err := c.RunSystemctlCommand(ctx, NewRunSystemctlCommand(
		WithSystemctlCommandSubCommand(SystemctlSubCommandListUnits),
		WithSystemctlCommandUnitType(UnitTypeService),
		WithSystemctlCommandOptions("--full --all --plain --no-legend"),
	))
	if err != nil {
		return nil, remoteflight.WrapErrorWith(err, res.Stderr, "listing services")
	}

	return parseServiceInfos(res.Stdout), nil
}

// RunSystemctlCommand runs a systemctl command request.
func (c *client) RunSystemctlCommand(ctx context.Context, req *SystemctlCommandReq) (*SystemctlCommandRes, error) {
	res := &SystemctlCommandRes{}
	cmd, err := req.String()
	if err != nil {
		return res, err
	}

	res.Stdout, res.Stderr, err = c.transport.Run(ctx, command.New(cmd, command.WithEnvVars(req.Env)))
	if err != nil {
		var exitError *it.ExecError
		if errors.As(err, &exitError) {
			res.Status = exitError.ExitCode()
		}
	}

	return res, err
}

func (c *client) EnableService(ctx context.Context, unit string) error {
	res, err := c.RunSystemctlCommand(ctx, NewRunSystemctlCommand(
		WithSystemctlCommandUnitName(unit),
		WithSystemctlCommandSubCommand(SystemctlSubCommandEnable),
		WithSystemctlCommandOptions("--now"),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, res.Stderr, fmt.Sprintf("enabling %s", unit))
	}

	return nil
}

func (c *client) StartService(ctx context.Context, unit string) error {
	res, err := c.RunSystemctlCommand(ctx, NewRunSystemctlCommand(
		WithSystemctlCommandUnitName(unit),
		WithSystemctlCommandSubCommand(SystemctlSubCommandStart),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, res.Stderr, fmt.Sprintf("starting %s", unit))
	}

	return nil
}

func (c *client) StopService(ctx context.Context, unit string) error {
	res, err := c.RunSystemctlCommand(ctx, NewRunSystemctlCommand(
		WithSystemctlCommandUnitName(unit),
		WithSystemctlCommandSubCommand(SystemctlSubCommandStop),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, res.Stderr, fmt.Sprintf("stopping %s", unit))
	}

	return nil
}

func (c *client) RestartService(ctx context.Context, unit string) error {
	isActive := c.IsActiveService(ctx, unit)

	// If it's already running smoothly stop it
	if isActive {
		err := c.StopService(ctx, unit)
		if err != nil {
			return err
		}
	} else {
		err := c.EnableService(ctx, unit)
		if err != nil {
			return err
		}
	}

	return c.StartService(ctx, unit)
}

// ServiceStatus returns the systemd status of the systemd service (until we have a better way).
func (c *client) ServiceStatus(ctx context.Context, unit string) StatusCode {
	res, err := c.RunSystemctlCommand(ctx, NewRunSystemctlCommand(
		WithSystemctlCommandUnitName(unit),
		WithSystemctlCommandSubCommand(SystemctlSubCommandIsActive),
	))
	// if we return no err, service is active
	if err == nil {
		return StatusActive
	}

	// otherwise, set status to Unknown by default and extract the code from xssh
	statusCode := StatusUnknown
	if res.Status != 0 {
		statusCode = StatusCode(res.Status)
	}

	return statusCode
}

func (c *client) IsActiveService(ctx context.Context, unit string) bool {
	status := c.ServiceStatus(ctx, unit)
	return status == StatusActive
}

// parseServiceInfos parses the systemctl services output into a slice of ServiceInfos.
func parseServiceInfos(services string) []ServiceInfo {
	serviceInfos := []ServiceInfo{}
	for _, line := range strings.Split(services, "\n") {
		if line == "" {
			continue
		}
		match := serviceInfoRegex.FindStringSubmatch(line)
		result := make(map[string]string)
		for i, name := range serviceInfoRegex.SubexpNames() {
			if i != 0 && name != "" {
				result[name] = match[i]
			}
		}
		serviceInfos = append(serviceInfos, ServiceInfo{
			Unit:        result["unit"],
			Load:        result["load"],
			Active:      result["active"],
			Sub:         result["sub"],
			Description: result["description"],
		})
	}

	return serviceInfos
}
