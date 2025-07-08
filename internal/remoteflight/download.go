// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package remoteflight

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp-forge/terraform-provider-enos/internal/retry"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
)

// DownloadRequest performs a remote flight control download.
type DownloadRequest struct {
	FlightControlPath string
	URL               string
	HTTPMethod        string
	Destination       string
	Mode              string
	SHA256            string
	Timeout           string
	AuthUser          string
	AuthPassword      string
	AuthToken         string
	Sudo              bool
	Replace           bool
	RetryOpts         []retry.RetrierOpt
}

// DownloadResponse is a flight control download response.
type DownloadResponse struct{}

// DownloadOpt is a functional option for an download request.
type DownloadOpt func(*DownloadRequest) *DownloadRequest

// NewDownloadRequest takes functional options and returns a new download request.
func NewDownloadRequest(opts ...DownloadOpt) *DownloadRequest {
	dr := &DownloadRequest{
		FlightControlPath: DefaultFlightControlPath,
		HTTPMethod:        "GET",
		Timeout:           "5m",
		Mode:              "0755",
		Sudo:              false,
		Replace:           false,
		RetryOpts: []retry.RetrierOpt{
			retry.WithMaxRetries(3),
			retry.WithIntervalFunc(retry.IntervalFibonacci(time.Second)),
		},
	}

	for _, opt := range opts {
		dr = opt(dr)
	}

	return dr
}

// WithDownloadRequestFlightControlPath sets the location of the enos-flight-contro
// binary.
func WithDownloadRequestFlightControlPath(path string) DownloadOpt {
	return func(dr *DownloadRequest) *DownloadRequest {
		dr.FlightControlPath = path
		return dr
	}
}

// WithDownloadRequestURL sets the download drL.
func WithDownloadRequestURL(url string) DownloadOpt {
	return func(dr *DownloadRequest) *DownloadRequest {
		dr.URL = url
		return dr
	}
}

// WithDownloadRequestHTTPMethod sets the download HTTP method.
func WithDownloadRequestHTTPMethod(meth string) DownloadOpt {
	return func(dr *DownloadRequest) *DownloadRequest {
		dr.HTTPMethod = meth
		return dr
	}
}

// WithDownloadRequestDestination sets destination path of the downloaded file.
func WithDownloadRequestDestination(dest string) DownloadOpt {
	return func(dr *DownloadRequest) *DownloadRequest {
		dr.Destination = dest
		return dr
	}
}

// WithDownloadRequestMode sets the mode for the downloaded file.
func WithDownloadRequestMode(mode string) DownloadOpt {
	return func(dr *DownloadRequest) *DownloadRequest {
		dr.Mode = mode
		return dr
	}
}

// WithDownloadRequestAuthUser sets basic auth user.
func WithDownloadRequestAuthUser(user string) DownloadOpt {
	return func(dr *DownloadRequest) *DownloadRequest {
		dr.AuthUser = user
		return dr
	}
}

// WithDownloadRequestAuthPassword sets basic auth password.
func WithDownloadRequestAuthPassword(password string) DownloadOpt {
	return func(dr *DownloadRequest) *DownloadRequest {
		dr.AuthPassword = password
		return dr
	}
}

// WithDownloadRequestAuthToken sets auth token.
func WithDownloadRequestAuthToken(token string) DownloadOpt {
	return func(dr *DownloadRequest) *DownloadRequest {
		dr.AuthToken = token
		return dr
	}
}

// WithDownloadRequestSHA256 sets required SHA256 sum.
func WithDownloadRequestSHA256(sha string) DownloadOpt {
	return func(dr *DownloadRequest) *DownloadRequest {
		dr.SHA256 = sha
		return dr
	}
}

// WithDownloadRequestUseSudo determines if the download command should be run with
// sudo.
func WithDownloadRequestUseSudo(useSudo bool) DownloadOpt {
	return func(dr *DownloadRequest) *DownloadRequest {
		dr.Sudo = useSudo
		return dr
	}
}

// WithDownloadRequestReplace determines if the download command should replace
// existing files.
func WithDownloadRequestReplace(replace bool) DownloadOpt {
	return func(dr *DownloadRequest) *DownloadRequest {
		dr.Replace = replace
		return dr
	}
}

// WithDownloadRequestRetryOptions sets retry options for dowload operation.
func WithDownloadRequestRetryOptions(opts ...retry.RetrierOpt) DownloadOpt {
	return func(dr *DownloadRequest) *DownloadRequest {
		dr.RetryOpts = opts
		return dr
	}
}

// Download downloads a file on a remote machine with enos-flight-control, retrying if necessary.
func Download(ctx context.Context, tr transport.Transport, dr *DownloadRequest) (*DownloadResponse, error) {
	res := &DownloadResponse{}

	select {
	case <-ctx.Done():
		return res, ctx.Err()
	default:
	}

	cmd := fmt.Sprintf("%s download --url '%s' --destination '%s' --mode '%s' --timeout '%s' --replace=%t",
		dr.FlightControlPath,
		dr.URL,
		dr.Destination,
		dr.Mode,
		dr.Timeout,
		dr.Replace,
	)
	if dr.Sudo {
		cmd = "sudo " + cmd
	}
	if dr.SHA256 != "" {
		cmd = fmt.Sprintf("%s --sha256 '%s'", cmd, dr.SHA256)
	}
	if dr.AuthUser != "" && dr.AuthPassword != "" {
		cmd = fmt.Sprintf("%s --auth-user '%s' --auth-password '%s'", cmd, dr.AuthUser, dr.AuthPassword)
	}

	if dr.AuthToken != "" {
		cmd = fmt.Sprintf("%s --auth-token '%s'", cmd, dr.AuthToken)
	}

	runCmd := func(ctx context.Context) (interface{}, error) {
		var resp interface{}
		stdout, stderr, err := tr.Run(ctx, command.New(cmd))
		if err != nil {
			return resp, WrapErrorWith(err, stdout, stderr)
		}

		return resp, err
	}

	opts := append(dr.RetryOpts, retry.WithRetrierFunc(runCmd))
	r, err := retry.NewRetrier(opts...)
	if err != nil {
		return res, err
	}

	_, err = retry.Retry(ctx, r)
	if err != nil {
		return res, err
	}

	return res, nil
}
