package download

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Request is a download request.
type Request struct {
	URL          *url.URL
	HTTPMethod   string
	Destination  io.WriteCloser
	SHA256       string
	AuthUser     string
	AuthPassword string
}

// RequestOpt are functional options for a new Request.
type RequestOpt func(*Request) (*Request, error)

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

	if res.StatusCode != http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("reading download query response body: %w", err)
		}

		return fmt.Errorf("download error: %s - %s", res.Status, string(body))
	}

	if req.SHA256 == "" {
		_, err = io.Copy(req.Destination, res.Body)
		return err
	}

	buf := bytes.Buffer{}
	fan := io.MultiWriter(req.Destination, &buf)
	_, err = io.Copy(fan, res.Body)
	if err != nil {
		return err
	}

	sha := fmt.Sprintf("%x", sha256.Sum256(buf.Bytes()))
	if sha != req.SHA256 {
		return fmt.Errorf("download failed: unxpected SHA 256 sum: expected (%s) received (%s)", req.SHA256, sha)
	}

	return nil
}
