// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package systemd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/log"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/remoteflight"
	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/file"
)

// KnownServices are list of known HashiCorp services.
var KnownServices = []string{"boundary", "consul", "vault"}

// Client an interface for a sysetmd client.
type Client interface {
	// CreateUnitFile creates a systemd unit file for the provided request
	CreateUnitFile(ctx context.Context, req *CreateUnitFileRequest) error
	// EnableService enables a systemd service with the provided unit name
	EnableService(ctx context.Context, unit string) error
	// GetUnitJournal gets the journal for a systemd unit
	GetUnitJournal(ctx context.Context, req *GetUnitJournalRequest) (remoteflight.GetLogsResponse, error)
	// ListServices gets the list of systemd services installed
	ListServices(ctx context.Context) ([]ServiceInfo, error)
	// RestartService restarts a systemd service with the provided unit name
	RestartService(ctx context.Context, unit string) error
	// RunSystemctlCommand runs a systemctl command for the provided request
	RunSystemctlCommand(ctx context.Context, req *SystemctlCommandReq) (*SystemctlCommandRes, error)
	// ShowProperties gets the properties of a matching unit or job
	ShowProperties(ctx context.Context, unit string) (UnitProperties, error)
	// StartService starts a systemd service with the provided unit name
	StartService(ctx context.Context, unit string) error
	// StopService stops a systemd service with the provided unit name
	StopService(ctx context.Context, unit string) error
	// ServiceStatus gets the service status for a systemd service with the provided unit name
	ServiceStatus(ctx context.Context, unit string) SystemctlStatusCode
}

type client struct {
	transport it.Transport
	logger    log.Logger
}

var _ Client = (*client)(nil)

// NewClient takes a transport and logger and returns a new systemd Client.
func NewClient(transport it.Transport, logger log.Logger) Client {
	return &client{
		transport: transport,
		logger:    logger,
	}
}

// GetUnitJournal gets the unit journal.
func (c *client) GetUnitJournal(ctx context.Context, req *GetUnitJournalRequest) (
	remoteflight.GetLogsResponse,
	error,
) {
	stdout, stderr, err := c.transport.Run(ctx, command.New("journalctl -x -u "+req.Unit))
	if err != nil {
		return nil, fmt.Errorf("failed to get systemd logs, due to: %w", err)
	}

	if stderr != "" {
		c.logger.With("stderr", stderr).Error("stderr retrieving systemd logs")
	}

	if stdout == "" {
		c.logger.Debug("no systemd logs")
	}

	return &GetUnitJournalResponse{
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
		return errors.New("you must provide a unit destination path")
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
	res := &SystemctlCommandRes{
		Status: StatusUnknown,
	}
	cmd, err := req.String()
	if err != nil {
		return res, err
	}

	res.Stdout, res.Stderr, err = c.transport.Run(ctx, command.New(cmd, command.WithEnvVars(req.Env)))
	if err != nil {
		var exitError *it.ExecError
		if errors.As(err, &exitError) {
			res.Status = SystemctlStatusCode(exitError.ExitCode())
		}
	} else {
		res.Status = StatusOK
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
		return errors.Join(fmt.Errorf("enabling %s, stderr: %s", unit, res.Stderr), err)
	}

	return nil
}

func (c *client) StartService(ctx context.Context, unit string) error {
	res, err := c.RunSystemctlCommand(ctx, NewRunSystemctlCommand(
		WithSystemctlCommandUnitName(unit),
		WithSystemctlCommandSubCommand(SystemctlSubCommandStart),
	))
	if err != nil {
		return errors.Join(fmt.Errorf("starting %s, stderr: %s", unit, res.Stderr), err)
	}

	return nil
}

func (c *client) StopService(ctx context.Context, unit string) error {
	res, err := c.RunSystemctlCommand(ctx, NewRunSystemctlCommand(
		WithSystemctlCommandUnitName(unit),
		WithSystemctlCommandSubCommand(SystemctlSubCommandStop),
	))
	if err != nil {
		return errors.Join(fmt.Errorf("stopping %s, stderr: %s", unit, res.Stderr), err)
	}

	return nil
}

func (c *client) RestartService(ctx context.Context, unit string) error {
	props, err := c.ShowProperties(ctx, unit)
	if err != nil {
		return fmt.Errorf("restarting %s, systemd properties: %s, %w", unit, props, err)
	}

	// Shut the service down if it's loaded and active. We don't really to worry
	// about our sub-state as we're trying to restart it.
	if props.HasProperties(UnitProperties{
		"LoadState":   "loaded",
		"ActiveState": "active",
	}) {
		err = c.StopService(ctx, unit)
		if err != nil {
			return fmt.Errorf("restarting %s, systemd properties: %s, %w", unit, props, err)
		}
	}

	// Enable our service if it isn't already
	if !props.HasProperties(UnitProperties{
		"UnitFileStatus": "enabled",
	}) {
		err = c.EnableService(ctx, unit)
		if err != nil {
			return fmt.Errorf("restarting %s, systemd properties: %s, %w", unit, props, err)
		}
	}

	// Start it
	err = c.StartService(ctx, unit)
	if err != nil {
		return fmt.Errorf("restarting %s, systemd properties: %s, %w", unit, props, err)
	}

	return nil
}

// ServiceStatus returns the systemd status of the systemd service (until we have a better way).
func (c *client) ServiceStatus(ctx context.Context, unit string) SystemctlStatusCode {
	res, _ := c.RunSystemctlCommand(ctx, NewRunSystemctlCommand(
		WithSystemctlCommandUnitName(unit),
		WithSystemctlCommandSubCommand(SystemctlSubCommandShow),
	))

	return res.Status
}

// ShowProperties gets a full list of systemd properties for a matching unit or job.
func (c *client) ShowProperties(ctx context.Context, unit string) (UnitProperties, error) {
	res, err := c.RunSystemctlCommand(ctx, NewRunSystemctlCommand(
		WithSystemctlCommandUnitName(unit),
		WithSystemctlCommandSubCommand(SystemctlSubCommandShow),
	))

	if res.Stdout == "" {
		err = errors.Join(err, errors.New("no show result was written to STDOUT"))
	}

	var props UnitProperties
	if err == nil {
		var err1 error
		props, err1 = decodeUnitPropertiesFromShow(res.Stdout)
		if err1 != nil {
			err = fmt.Errorf("%w: properties: %s", err1, props)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("showing systemd properties: %w", err)
	}

	return props, nil
}

func decodeUnitPropertiesFromShow(show string) (UnitProperties, error) {
	var err error
	props := NewUnitProperties()

	scanner := bufio.NewScanner(strings.NewReader(show))
	for scanner.Scan() {
		if e := scanner.Err(); e != nil {
			err = errors.Join(err, e)
			continue
		}

		if t := scanner.Text(); t != "" {
			parts := strings.SplitN(t, "=", 2)
			if len(parts) != 2 {
				continue
			}
			props[parts[0]] = parts[1]
		}
	}

	if err != nil {
		return nil, fmt.Errorf("decoding properties from show output: %w", err)
	}

	return props, nil
}
