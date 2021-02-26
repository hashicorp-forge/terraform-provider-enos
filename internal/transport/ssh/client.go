package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/hashicorp/go-multierror"
)

type client struct {
	agentConn       io.Closer
	client          *xssh.Client
	clientConfig    *xssh.ClientConfig
	session         *xssh.Session
	keepaliveCancel context.CancelFunc
	keepaliveErrC   chan error
	transportCfg    *transportCfg
}

type transportCfg struct {
	user           string
	host           string
	key            string
	keyPath        string
	passphrase     string
	passphrasePath string
	password       string
	port           string
}

func (c *client) parseKey(ctx context.Context, key string) (xssh.AuthMethod, error) {
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

func (c *client) parseKeyWithPassphrase(ctx context.Context, key, passphrase string) (xssh.AuthMethod, error) {
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

func (c *client) readFile(ctx context.Context, path string) (string, error) {
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

func (c *client) init(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	var err error

	c.clientConfig = &xssh.ClientConfig{
		Config:          xssh.Config{},
		User:            c.transportCfg.user,
		HostKeyCallback: xssh.InsecureIgnoreHostKey(),
		Auth:            []xssh.AuthMethod{},
	}

	c.clientConfig.Config.SetDefaults() // Use the default ciphers and key exchanges

	if c.transportCfg.password != "" {
		c.clientConfig.Auth = append(c.clientConfig.Auth, xssh.Password(c.transportCfg.password))
	}

	key := c.transportCfg.key
	if c.transportCfg.keyPath != "" {
		key, err = c.readFile(ctx, c.transportCfg.keyPath)
		if err != nil {
			return err
		}
	}

	passphrase := c.transportCfg.passphrase
	if c.transportCfg.passphrasePath != "" {
		passphrase, err = c.readFile(ctx, c.transportCfg.passphrasePath)
		if err != nil {
			return err
		}
	}

	if key != "" {
		var auth xssh.AuthMethod

		if passphrase != "" {
			auth, err = c.parseKeyWithPassphrase(ctx, key, passphrase)
		} else {
			auth, err = c.parseKey(ctx, key)
		}
		if err != nil {
			return err
		}
		c.clientConfig.Auth = append(c.clientConfig.Auth, auth)
	}

	return nil
}

func (c *client) connect(ctx context.Context) error {
	var err error

	sshAgentConn, sshAgent, ok := c.connectSSHAgent()
	if ok {
		c.clientConfig.Auth = append(c.clientConfig.Auth, sshAgent)
		c.agentConn = sshAgentConn
	}

	// We've seen races where we attempt to dial the target machine before sshd
	// is up and instead of an eventual connection we instead spend the entire
	// duration of the Timeout doing nothing but churning CPU and then fail.
	// Instead of a one-shot dial with a long timeout, we'll retry with multiple
	// times with progressively longer timeouts for a total of 15 seconds.
	for attempt := 1; attempt < 5; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		c.clientConfig.Timeout = time.Duration(attempt) * time.Second
		c.client, err = xssh.Dial("tcp", net.JoinHostPort(c.transportCfg.host, c.transportCfg.port), c.clientConfig)
		if err == nil {
			break
		}

		var sysErr syscall.Errno
		if errors.As(err, &sysErr) {
			switch sysErr {
			case syscall.ECONNREFUSED, syscall.ECONNABORTED, syscall.ECONNRESET:
				// If the connection was refused we didn't wait the duration of
				// the timeout so we'll sleep it.
				time.Sleep(c.clientConfig.Timeout)
			default:
			}
		}

		c.client = nil
	}

	if err != nil {
		return fmt.Errorf("timed out dialing %s:%s: %w", c.transportCfg.host, c.transportCfg.port, err)
	}

	c.keepaliveErrC = make(chan error, 1)
	keepaliveCtx, keepaliveCancel := context.WithCancel(ctx)
	c.keepaliveCancel = keepaliveCancel
	go c.startConnectionKeepalive(keepaliveCtx)
	return nil
}

func (c *client) startConnectionKeepalive(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	errCount := 0
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			c.keepaliveErrC <- ctx.Err()
			return
		default:
		}

		select {
		case <-t.C:
			ok, _, err := c.client.SendRequest("enos@hashicorp.io", true, nil)
			if !ok {
				errCount++
			}

			if errCount == 5 {
				c.keepaliveErrC <- err
				return
			}
		case <-ctx.Done():
			c.keepaliveErrC <- ctx.Err()
			return
		}
	}
}

func (c *client) Close() error {
	var err error
	merr := &multierror.Error{}

	select {
	case err = <-c.keepaliveErrC:
		merr = multierror.Append(merr, err)
	default:
	}

	if c.keepaliveCancel != nil {
		c.keepaliveCancel()
	}
	c.keepaliveErrC = nil

	merr = multierror.Append(merr, c.closeSession())

	if c.client != nil {
		err := c.client.Close()
		c.client = nil
		if !errors.Is(err, io.EOF) {
			merr = multierror.Append(merr, err)
		}
	}

	if c.agentConn != nil {
		err = c.agentConn.Close()
		c.agentConn = nil
		if !errors.Is(err, io.EOF) {
			merr = multierror.Append(merr, err)
		}
	}

	return merr.ErrorOrNil()
}

func (c *client) closeSession() error {
	var err error

	if c.session != nil {
		err = c.session.Close()
		c.session = nil
	}

	if errors.Is(err, io.EOF) {
		return nil
	}

	return err
}

func (c *client) startSession(ctx context.Context) error {
	var err error

	if c.client == nil {
		err = c.connect(ctx)
		if err != nil {
			return fmt.Errorf("creating client %w", err)
		}
	}

	c.session, err = c.client.NewSession()
	if err != nil {
		return fmt.Errorf("creating ssh session %w", err)
	}

	return nil
}

func (c *client) connectSSHAgent() (net.Conn, xssh.AuthMethod, bool) {
	var auth xssh.AuthMethod

	sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return sshAgent, auth, false
	}

	return sshAgent, xssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers), true
}
