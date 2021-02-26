package ssh

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	xssh "golang.org/x/crypto/ssh"

	it "github.com/hashicorp/enos-provider/internal/transport"
)

// Opt is a functional option for the SSH transport
type Opt func(*transport)

type transport struct {
	client *client
	ctx    context.Context
}

var _ it.Transport = (*transport)(nil)

// New takes zero or more functional options and return a new transport
func New(opts ...Opt) (it.Transport, error) {
	t := &transport{
		client: &client{
			clientConfig: &xssh.ClientConfig{},
			transportCfg: &transportCfg{
				port: "22",
			},
		},
		ctx: context.Background(),
	}
	for _, opt := range opts {
		opt(t)
	}

	return t, t.client.init(t.ctx)
}

// WithUser sets the user
func WithUser(u string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.user = u
	}
}

// WithHost sets the host
func WithHost(h string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.host = h
	}
}

// WithKey sets the key
func WithKey(k string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.key = k
	}
}

// WithKeyPath sets the key path
func WithKeyPath(p string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.keyPath = p
	}
}

// WithPassphrase sets the key passphrase
func WithPassphrase(p string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.passphrase = p
	}
}

// WithPassphrasePath sets the key passphrase path
func WithPassphrasePath(p string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.passphrasePath = p
	}
}

// WithPassword sets the password
func WithPassword(p string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.password = p
	}
}

// WithPort sets the port
func WithPort(p string) func(*transport) {
	return func(t *transport) {
		t.client.transportCfg.port = p
	}
}

// WithContext sets the context to use when initializing the resources
func WithContext(ctx context.Context) func(*transport) {
	return func(t *transport) {
		t.ctx = ctx
	}
}

// Copy copies the source to the destination using the given SSH configuration
// options.
func (t *transport) Copy(ctx context.Context, src it.Copyable, dst string) (err error) {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	err = t.client.startSession(ctx)
	defer func() { err = t.client.closeSession() }()
	if err != nil {
		return err
	}

	stdin, err := t.client.session.StdinPipe()
	if err != nil {
		return fmt.Errorf("creating SSH STDIN pipe: %w", err)
	}
	errC := make(chan error, 1)

	copyFile := func() {
		_, err := fmt.Fprintln(stdin, "C0644", src.Size(), filepath.Base(dst))
		if err != nil {
			errC <- fmt.Errorf("writing file header: %w", err)
			return
		}

		if src.Size() > 0 {
			_, err = io.Copy(stdin, src)
			if err != nil {
				errC <- fmt.Errorf("writing file: %w", err)
				return
			}
		}

		_, err = fmt.Fprint(stdin, "\x00")
		if err != nil {
			errC <- fmt.Errorf("writing end of file: %w", err)
			return

		}
		errC <- nil

		stdin.Close()
	}

	go copyFile()

	err = t.client.session.Run(fmt.Sprintf("scp -tr %s", dst))
	if err != nil {
		return fmt.Errorf("starting scp: %w", err)
	}

	select {
	case <-ctx.Done():
		err = ctx.Err()
		return err
	case err = <-errC:
		return err
	}
}

// Stream runs the given command and returns readers for STDOUT and STDERR and
// a err channel where any encountered errors are streamed.
func (t *transport) Stream(ctx context.Context, cmd it.Command) (stdout io.Reader, stderr io.Reader, errC chan error) {
	var err error
	errC = make(chan error, 3)

	select {
	case <-ctx.Done():
		errC <- ctx.Err()
		return stdout, stderr, errC
	default:
	}

	disconnect := func() {
		err = t.client.closeSession()
		if err != nil {
			errC <- err
		}
	}

	err = t.client.startSession(ctx)
	if err != nil {
		defer disconnect()
		errC <- err
		return stdout, stderr, errC
	}

	stdout, err = t.client.session.StdoutPipe()
	if err != nil {
		defer disconnect()
		errC <- err
		return stdout, stderr, errC
	}

	stderr, err = t.client.session.StderrPipe()
	if err != nil {
		defer disconnect()
		errC <- err
		return stdout, stderr, errC
	}

	err = t.client.session.Start(cmd.Cmd())
	if err != nil {
		defer disconnect()
		errC <- err
		return stdout, stderr, errC
	}

	waitForCommandToFinish := func() {
		defer disconnect()
		errC <- t.client.session.Wait()
	}

	go waitForCommandToFinish()

	return stdout, stderr, errC
}

// Run runs the command and returns STDOUT, STDERR and and the first error encountered
func (t *transport) Run(ctx context.Context, cmd it.Command) (string, string, error) {
	var err error

	stdout, stderr, errC := t.Stream(ctx, cmd)

	select {
	case <-ctx.Done():
		return "", "", ctx.Err()
	case err = <-errC:
		return "", "", err
	default:
	}

	captureWait := sync.WaitGroup{}
	captureWait.Add(2)

	captureOutput := func(in io.Reader, out *bytes.Buffer) {
		scanner := bufio.NewScanner(in)

		for scanner.Scan() {
			out.WriteString(scanner.Text())
		}

		captureWait.Done()
	}

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	go captureOutput(stdout, stdoutBuf)
	go captureOutput(stderr, stderrBuf)

	select {
	case <-ctx.Done():
		captureWait.Wait()
		return stdoutBuf.String(), stderrBuf.String(), ctx.Err()
	case err = <-errC:
		captureWait.Wait()
		return stdoutBuf.String(), stderrBuf.String(), err
	}
}

// Close closes any underlying connections
func (t *transport) Close() error {
	if t.client == nil {
		return nil
	}

	return t.client.Close()
}
