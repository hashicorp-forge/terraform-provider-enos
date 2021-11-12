package consul

import (
	"context"

	"github.com/hashicorp/enos-provider/internal/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
)

// Enable enables the consul service
func Enable(ctx context.Context, ssh it.Transport) error {
	res, err := remoteflight.RunSystemctlCommand(ctx, ssh, remoteflight.NewRunSystemctlCommand(
		remoteflight.WithSystemctlCommandUnitName("consul"),
		remoteflight.WithSystemctlCommandSubCommand(remoteflight.SystemctlSubCommandEnable),
		remoteflight.WithSystemctlCommandOptions("--now"),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, res.Stderr, "enabling consul")
	}

	return nil
}

// Start starts the consul service
func Start(ctx context.Context, ssh it.Transport) error {
	res, err := remoteflight.RunSystemctlCommand(ctx, ssh, remoteflight.NewRunSystemctlCommand(
		remoteflight.WithSystemctlCommandUnitName("consul"),
		remoteflight.WithSystemctlCommandSubCommand(remoteflight.SystemctlSubCommandStart),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, res.Stderr, "starting consul")
	}

	return nil
}

// Stop stops the consul service
func Stop(ctx context.Context, ssh it.Transport) error {
	res, err := remoteflight.RunSystemctlCommand(ctx, ssh, remoteflight.NewRunSystemctlCommand(
		remoteflight.WithSystemctlCommandUnitName("consul"),
		remoteflight.WithSystemctlCommandSubCommand(remoteflight.SystemctlSubCommandStop),
	))
	if err != nil {
		return remoteflight.WrapErrorWith(err, res.Stderr, "stopping consul")
	}

	return nil
}

// Restart restarts the consul service
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
