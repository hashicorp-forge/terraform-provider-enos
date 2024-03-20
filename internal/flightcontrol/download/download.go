// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package download

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// Request is a download request.
type Request struct {
	URL          *url.URL
	HTTPMethod   string
	Destination  io.WriteCloser
	SHA256       string
	AuthUser     string
	AuthPassword string
	WriteStdout  bool
}

// RequestOpt are functional options for a new Request.
type RequestOpt func(*Request) (*Request, error)

// ErrDownload is a download error.
type ErrDownload struct {
	Err        error
	StatusCode int
}

// Error returns the error as a string.
func (e *ErrDownload) Error() string {
	return fmt.Sprintf("%s (%d)", e.Err, e.StatusCode)
}

// Unwrap returns the wrapped error.
func (e *ErrDownload) Unwrap() error {
	return e.Err
}

// WithRequestDestination sets the destination.
func WithRequestDestination(dst io.WriteCloser) RequestOpt {
	return func(req *Request) (*Request, error) {
		req.Destination = dst
		return req, nil
	}
}

// WithRequestURL sets the source download URL.
func WithRequestURL(u string) RequestOpt {
	return func(req *Request) (*Request, error) {
		var err error
		req.URL, err = url.Parse(u)

		return req, err
	}
}

// WithRequestSHA256 sets the required SHA 256 sum.
func WithRequestSHA256(sha string) RequestOpt {
	return func(req *Request) (*Request, error) {
		req.SHA256 = sha

		return req, nil
	}
}

// WithRequestAuthUser sets the basic auth user.
func WithRequestAuthUser(user string) RequestOpt {
	return func(req *Request) (*Request, error) {
		req.AuthUser = user

		return req, nil
	}
}

// WithRequestAuthPassword sets the basic auth password.
func WithRequestAuthPassword(password string) RequestOpt {
	return func(req *Request) (*Request, error) {
		req.AuthPassword = password

		return req, nil
	}
}

// WithRequestWriteStdout sets whether or not we should write the body to STDOUT.
func WithRequestWriteStdout(enabled bool) RequestOpt {
	return func(req *Request) (*Request, error) {
		req.WriteStdout = enabled

		return req, nil
	}
}

// NewRequest takes N RequestOpt args and returns a new request.
func NewRequest(opts ...RequestOpt) (*Request, error) {
	r := &Request{
		HTTPMethod: http.MethodGet,
	}

	for _, opt := range opts {
		r, err := opt(r)
		if err != nil {
			return r, err
		}
	}

	return r, nil
}

// Download takes and context and request and performs the download request
// and optionally verifies the downloaded file SHA 256 sum.
func Download(ctx context.Context, req *Request) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	in := bytes.Buffer{}
	dreq, err := http.NewRequestWithContext(ctx, req.HTTPMethod, req.URL.String(), &in)
	if err != nil {
		return err
	}

	if req.AuthUser != "" && req.AuthPassword != "" {
		dreq.SetBasicAuth(req.AuthUser, req.AuthPassword)
	}

	res, err := http.DefaultClient.Do(dreq)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var memBuf io.Writer
	if req.SHA256 != "" || (res.StatusCode != http.StatusOK) {
		memBuf = new(bytes.Buffer)
	} else {
		memBuf = io.Discard
	}

	outs := []io.Writer{memBuf}

	if req.WriteStdout {
		outs = append(outs, os.Stdout)
	}

	if (res.StatusCode == http.StatusOK) && (req.Destination != nil) {
		outs = append(outs, req.Destination)
	}

	fan := io.MultiWriter(outs...)
	_, err = io.Copy(fan, res.Body)

	var resBody []byte
	buf, ok := memBuf.(*bytes.Buffer)
	if ok {
		resBody = buf.Bytes()
	}

	if res.StatusCode != http.StatusOK {
		err = errors.Join(err, fmt.Errorf("download error: %s - %s", res.Status, string(resBody)))
	}

	if req.SHA256 != "" {
		sha := fmt.Sprintf("%x", sha256.Sum256(resBody))
		if sha != req.SHA256 {
			err = errors.Join(err, fmt.Errorf("download failed: unxpected SHA 256 sum: expected (%s) received (%s)", req.SHA256, sha))
		}
	}

	if err != nil {
		return &ErrDownload{Err: err, StatusCode: res.StatusCode}
	}

	return nil
}
