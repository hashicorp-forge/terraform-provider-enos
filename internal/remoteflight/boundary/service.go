package boundary

import (
	"context"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
)

// Enable enables the boundary service
func Enable(ctx context.Context, ssh it.Transport) error {
	res, err := remoteflight.RunSystemctlCommand(ctx, ssh, remoteflight.NewRunSystemctlCommand(
		remoteflight.WithSystemctlCommandUnitName("boundary"),
		remoteflight.WithSystemctlCommandSubCommand(remoteflight.SystemctlSubCommandEnable),
		remoteflight.WithSystemctlCommandOptions("--now"),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, res.Stderr, "enabling boundary")
	}

	return nil
}

// Start starts the boundary service
func Start(ctx context.Context, ssh it.Transport) error {
	res, err := remoteflight.RunSystemctlCommand(ctx, ssh, remoteflight.NewRunSystemctlCommand(
		remoteflight.WithSystemctlCommandUnitName("boundary"),
		remoteflight.WithSystemctlCommandSubCommand(remoteflight.SystemctlSubCommandStart),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, res.Stderr, "starting boundary")
	}

	return nil
}

// Stop stops the boundary service
func Stop(ctx context.Context, ssh it.Transport) error {
	res, err := remoteflight.RunSystemctlCommand(ctx, ssh, remoteflight.NewRunSystemctlCommand(
		remoteflight.WithSystemctlCommandUnitName("boundary"),
		remoteflight.WithSystemctlCommandSubCommand(remoteflight.SystemctlSubCommandStop),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, res.Stderr, "stopping boundary")
	}

	return nil
}

// Restart restarts the boundary service
func Restart(ctx context.Context, ssh it.Transport) error {
	_, err := Status(ctx, ssh, "boundary")

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
