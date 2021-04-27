package remoteflight

import (
	"context"
	"fmt"

	"github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// UnzipRequest performs a remote flight control unzip
type UnzipRequest struct {
	FlightControlPath          string
	SourcePath                 string
	DestinationDirectory       string
	FileMode                   string
	DestinationDirectoryMode   string
	Sudo                       bool
	CreateDestinationDirectory bool
}

// UnzipResponse is a flight control unzip response
type UnzipResponse struct{}

// UnzipOpt is a functional option for an unzip request
type UnzipOpt func(*UnzipRequest) *UnzipRequest

// NewUnzipRequest takes functional options and returns a new unzip request
func NewUnzipRequest(opts ...UnzipOpt) *UnzipRequest {
	var err error
	ur := &UnzipRequest{
		FlightControlPath:          DefaultPath,
		FileMode:                   "0755",
		DestinationDirectoryMode:   "0755",
		Sudo:                       false,
		CreateDestinationDirectory: true,
	}

	for _, opt := range opts {
		ur = opt(ur)
		if err != nil {
			return ur
		}
	}

	return ur
}

// WithUnzipRequestFlightControlPath sets the location of the enos-flight-contro
// binary
func WithUnzipRequestFlightControlPath(path string) UnzipOpt {
	return func(ur *UnzipRequest) *UnzipRequest {
		ur.FlightControlPath = path
		return ur
	}
}

// WithUnzipRequestSourcePath sets the zip archive source path
func WithUnzipRequestSourcePath(path string) UnzipOpt {
	return func(ur *UnzipRequest) *UnzipRequest {
		ur.SourcePath = path
		return ur
	}
}

// WithUnzipRequestDestinationDir sets the unzip directory
func WithUnzipRequestDestinationDir(dir string) UnzipOpt {
	return func(ur *UnzipRequest) *UnzipRequest {
		ur.DestinationDirectory = dir
		return ur
	}
}

// WithUnzipRequestFileMode sets the mode for files that are expanded
func WithUnzipRequestFileMode(mode string) UnzipOpt {
	return func(ur *UnzipRequest) *UnzipRequest {
		ur.FileMode = mode
		return ur
	}
}

// WithUnzipRequestDestinationDirMode sets the mode for destination directory
// if it is created
func WithUnzipRequestDestinationDirMode(mode string) UnzipOpt {
	return func(ur *UnzipRequest) *UnzipRequest {
		ur.DestinationDirectoryMode = mode
		return ur
	}
}

// WithUnzipRequestCreateDestinationDir determines if the destination directory
// should be created
func WithUnzipRequestCreateDestinationDir(create bool) UnzipOpt {
	return func(ur *UnzipRequest) *UnzipRequest {
		ur.CreateDestinationDirectory = create
		return ur
	}
}

// WithUnzipRequestUseSudo determines if the unzip command should be run with
// sudo
func WithUnzipRequestUseSudo(useSudo bool) UnzipOpt {
	return func(ur *UnzipRequest) *UnzipRequest {
		ur.Sudo = useSudo
		return ur
	}
}

// Unzip unzips an archive on a remote machine with enos-flight-control
func Unzip(ctx context.Context, ssh transport.Transport, ur *UnzipRequest) (*UnzipResponse, error) {
	res := &UnzipResponse{}

	select {
	case <-ctx.Done():
		return res, ctx.Err()
	default:
	}

	cmd := fmt.Sprintf("%s unzip --source %s --destination %s --mode %s --destination-mode %s --create-destination %t",
		ur.FlightControlPath,
		ur.SourcePath,
		ur.DestinationDirectory,
		ur.FileMode,
		ur.DestinationDirectoryMode,
		ur.CreateDestinationDirectory,
	)
	if ur.Sudo {
		cmd = fmt.Sprintf("sudo %s", cmd)
	}
	_, _, err := ssh.Run(ctx, command.New(cmd))

	return res, err
}
