package vault

import (
	"context"

	"github.com/hashicorp/enos-provider/internal/flightcontrol/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
)

// Enable enables the vault service
func Enable(ctx context.Context, ssh it.Transport) error {
	res, err := remoteflight.RunSystemctlCommand(ctx, ssh, remoteflight.NewRunSystemctlCommand(
		remoteflight.WithSystemctlCommandUnitName("vault"),
		remoteflight.WithSystemctlCommandSubCommand(remoteflight.SystemctlSubCommandEnable),
		remoteflight.WithSystemctlCommandOptions("--now"),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, res.Stderr, "enabling vault")
	}

	return nil
}

// Start starts the vault service
func Start(ctx context.Context, ssh it.Transport) error {
	res, err := remoteflight.RunSystemctlCommand(ctx, ssh, remoteflight.NewRunSystemctlCommand(
		remoteflight.WithSystemctlCommandUnitName("vault"),
		remoteflight.WithSystemctlCommandSubCommand(remoteflight.SystemctlSubCommandStart),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, res.Stderr, "starting vault")
	}

	return nil
}

// Stop stops the vault service
func Stop(ctx context.Context, ssh it.Transport) error {
	res, err := remoteflight.RunSystemctlCommand(ctx, ssh, remoteflight.NewRunSystemctlCommand(
		remoteflight.WithSystemctlCommandUnitName("vault"),
		remoteflight.WithSystemctlCommandSubCommand(remoteflight.SystemctlSubCommandStop),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, res.Stderr, "stopping vault")
	}

	return nil
}

// Restart restarts the vault service
func Restart(ctx context.Context, ssh it.Transport, req *StatusRequest) error {
	_, err := Status(ctx, ssh, req)

	// If it's already running smoothly stop it
	if err == nil {
		err = Stop(ctx, ssh)
		if err != nil {
			return err
		}
	} else {
		err = Enable(ctx, ssh)
		if err != nil {
			return err
		}
	}

	return Start(ctx, ssh)
}
