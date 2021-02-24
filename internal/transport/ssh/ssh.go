package ssh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	it "github.com/hashicorp/enos-provider/internal/transport"
)

// Opt is a functional option for the SSH transport
type Opt func(*transport)

type transport struct {
	user           string
	host           string
	key            string
	keyPath        string
	passphrase     string
	passphrasePath string
	password       string
	port           string
	ctx            context.Context
	clientConfig   *xssh.ClientConfig
}

var _ it.Transport = (*transport)(nil)

// New takes zero or more functional options and return a new transport
func New(opts ...Opt) (it.Transport, error) {
	t := &transport{
		ctx:  context.Background(),
		port: "22",
	}
	for _, opt := range opts {
		opt(t)
	}

	return t, t.init(t.ctx)
}

// WithUser sets the user
func WithUser(u string) func(*transport) {
	return func(t *transport) {
		t.user = u
	}
}

// WithHost sets the host
func WithHost(h string) func(*transport) {
	return func(t *transport) {
		t.host = h
	}
}

// WithKey sets the key
func WithKey(k string) func(*transport) {
	return func(t *transport) {
		t.key = k
	}
}

// WithKeyPath sets the key path
func WithKeyPath(p string) func(*transport) {
	return func(t *transport) {
		t.keyPath = p
	}
}

// WithPassphrase sets the key passphrase
func WithPassphrase(p string) func(*transport) {
	return func(t *transport) {
		t.passphrase = p
	}
}

// WithPassphrasePath sets the key passphrase path
func WithPassphrasePath(p string) func(*transport) {
	return func(t *transport) {
		t.passphrasePath = p
	}
}

// WithPassword sets the password
func WithPassword(p string) func(*transport) {
	return func(t *transport) {
		t.password = p
	}
}

// WithPort sets the port
func WithPort(p string) func(*transport) {
	return func(t *transport) {
		t.port = p
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
func (t *transport) Copy(ctx context.Context, src it.Copyable, dst string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	session, cleanup, err := t.newSession(ctx)
	if err != nil {
		return err
	}
	defer cleanup() // nolint: errcheck
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("creating SSH STDIN pipe: %w", err)
	}

	copySrc := func(errC chan error) {
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
	}

	errC := make(chan error, 1)
	go func() {
		copySrc(errC)
		stdin.Close()
	}()

	err = session.Run(fmt.Sprintf("scp -tr %s", dst))
	if err != nil {
		return fmt.Errorf("starting scp: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err = <-errC:
		return err
	}
}

func (t *transport) Run(ctx context.Context, cmd string) error {
	// TODO: implement this
	return nil
}

func (t *transport) Stream(ctx context.Context, cmd string) (stdout io.Reader, stderr io.Reader, err error) {
	// TODO: implement this
	return strings.NewReader("out"), strings.NewReader("err"), nil
}

func (t *transport) newSession(ctx context.Context) (*xssh.Session, func() error, error) {
	var client *xssh.Client
	var session *xssh.Session
	var err error
	var cleanup = func() error { return nil }

	if t.clientConfig == nil {
		err = t.init(ctx)
		if err != nil {
			return session, cleanup, fmt.Errorf("creating SSH client config: %w", err)
		}
	}

	conn, sshAgent, ok := t.sshAgent()
	if ok {
		t.clientConfig.Auth = append(t.clientConfig.Auth, sshAgent)
		cleanup = conn.Close
	}

	client, err = xssh.Dial("tcp", net.JoinHostPort(t.host, t.port), t.clientConfig)
	if err != nil {
		return session, cleanup, fmt.Errorf("dialing %s:%s: %w", t.host, t.port, err)
	}

	session, err = client.NewSession()
	if err != nil {
		return session, cleanup, fmt.Errorf("creating ssh session %w", err)
	}

	return session, cleanup, err
}

func (t *transport) parseKey(ctx context.Context, key string) (xssh.AuthMethod, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	signer, err := xssh.ParsePrivateKey([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}

	return xssh.PublicKeys(signer), nil
}

func (t *transport) parseKeyWithPassphrase(ctx context.Context, key, passphrase string) (xssh.AuthMethod, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	signer, err := xssh.ParsePrivateKeyWithPassphrase([]byte(key), []byte(passphrase))
	if err != nil {
		return nil, fmt.Errorf("parsing private key with passphrase: %w", err)
	}

	return xssh.PublicKeys(signer), nil
}

func (t *transport) readFile(ctx context.Context, path string) (string, error) {
	res := ""
	select {
	case <-ctx.Done():
		return res, ctx.Err()
	default:
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return res, err
	}

	handle, err := os.Open(abs)
	defer handle.Close() // nolint: staticcheck
	if err != nil {
		return res, err
	}

	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(handle)
	if err != nil {
		return res, err
	}

	return strings.TrimSpace(buf.String()), nil
}

func (t *transport) init(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	var err error

	t.clientConfig = &xssh.ClientConfig{
		Config:          xssh.Config{},
		User:            t.user,
		HostKeyCallback: xssh.InsecureIgnoreHostKey(),
		Auth:            []xssh.AuthMethod{},
	}

	t.clientConfig.Config.SetDefaults() // Use the default ciphers and key exchanges

	deadline, ok := ctx.Deadline()
	if ok {
		t.clientConfig.Timeout = time.Until(deadline)
	}

	if t.password != "" {
		t.clientConfig.Auth = append(t.clientConfig.Auth, xssh.Password(t.password))
	}

	key := t.key
	if t.keyPath != "" {
		key, err = t.readFile(ctx, t.keyPath)
		if err != nil {
			return err
		}
	}

	passphrase := t.passphrase
	if t.passphrasePath != "" {
		passphrase, err = t.readFile(ctx, t.passphrasePath)
		if err != nil {
			return err
		}
	}

	if key != "" {
		var auth xssh.AuthMethod

		if passphrase != "" {
			auth, err = t.parseKeyWithPassphrase(ctx, key, passphrase)
		} else {
			auth, err = t.parseKey(ctx, key)
		}
		if err != nil {
			return err
		}
		t.clientConfig.Auth = append(t.clientConfig.Auth, auth)
	}

	return err
}

func (t *transport) sshAgent() (net.Conn, xssh.AuthMethod, bool) {
	var auth xssh.AuthMethod

	sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return sshAgent, auth, false
	}

	return sshAgent, xssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers), true
}
