// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	"sync"
	"time"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/hashicorp/go-multierror"
)

type client struct {
	agentConn       io.Closer
	client          *xssh.Client
	clientConfig    *xssh.ClientConfig
	keepaliveCancel context.CancelFunc
	keepaliveErrC   chan error
	transportCfg    *transportCfg
	mu              sync.Mutex
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
	if err != nil {
		return res, err
	}
	defer handle.Close()

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

	c.mu = sync.Mutex{}
	c.client = nil
	c.keepaliveErrC = make(chan error, 1)

	c.clientConfig = &xssh.ClientConfig{
		Config: xssh.Config{},
		User:   c.transportCfg.user,
		//nolint:gosec// it's okay to ignore our host key
		HostKeyCallback: xssh.InsecureIgnoreHostKey(),
		Auth:            []xssh.AuthMethod{},
	}

	c.clientConfig.SetDefaults() // Use the default ciphers and key exchanges

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

// Connect creates a TCP connection to the SSH server, initiates an SSH
// handshake, creates an SSH client, and starts a background TCP keepalive
// routine.  It will retry the TCP connection in 1 second increments until
// either 60 seconds have elapsed or the context is done.
func (c *client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error

	sshAgentConn, sshAgent, ok := c.connectSSHAgent(ctx)
	if ok {
		c.clientConfig.Auth = append(c.clientConfig.Auth, sshAgent)
		c.agentConn = sshAgentConn
	}

	wrapErr := func(err error) error {
		return fmt.Errorf("timed out dialing %s:%s: %w", c.transportCfg.host, c.transportCfg.port, err)
	}

	// We've seen races where we attempt to dial the target machine before sshd
	// is up and instead of an eventual connection we instead spend the entire
	// duration of the client timeout doing nothing but churning CPU before
	// an eventual failure. Instead of a one-shot dial with a long timeout,
	// we'll fire off dial attempts every 3 seconds and get the first client
	// that succeeds.

	dialTimeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	dialTicker := time.NewTicker(3 * time.Second)

	dialErrs := make(chan error, 5)
	clientC := make(chan *xssh.Client)
	c.clientConfig.Timeout = 2 * time.Second
	dial := func() {
		client, err := xssh.Dial("tcp", net.JoinHostPort(c.transportCfg.host, c.transportCfg.port), c.clientConfig)
		if err == nil {
			dialTicker.Stop()
			clientC <- client

			return
		}
		dialErrs <- err
	}

	drainErrors := func(err error) error {
		merr := &multierror.Error{}
		merr = multierror.Append(merr, err)

		for {
			select {
			case err = <-dialErrs:
				merr = multierror.Append(merr, err)
			default:
				return merr.ErrorOrNil()
			}
		}
	}

	go dial()

	// waitForClientConnection waits for a client connection. Always make sure
	// to return the real nil here as xssh.Dial can return a net.Conn that has
	// a default value stored in a nil interface, which breaks if x == nil checks.
	// Thanks Go, neat feature (https://golang.org/doc/faq#nil_error)
	waitForClientConnection := func() (*xssh.Client, error) {
		defer dialTicker.Stop()
		for {
			// Always make sure we haven't hit our timeouts before we attempt another
			// dial.
			select {
			case <-ctx.Done():
				return nil, wrapErr(drainErrors(ctx.Err()))
			case <-dialTimeout.Done():
				return nil, wrapErr(drainErrors(errors.New("exceeded client wait connection limit")))
			default:
			}

			select {
			case <-ctx.Done():
				return nil, wrapErr(drainErrors(ctx.Err()))
			case <-dialTimeout.Done():
				return nil, wrapErr(drainErrors(errors.New("exceeded client wait connection limit")))
			case <-dialTicker.C:
				go dial()
			case client := <-clientC:
				return client, nil
			}
		}
	}
	c.client, err = waitForClientConnection()
	if err != nil {
		return err
	}

	keepaliveCtx, keepaliveCancel := context.WithCancel(ctx)
	c.keepaliveCancel = keepaliveCancel

	startConnectionKeepalive := func(ctx context.Context, client *xssh.Client) {
		// get a copy of the client so we don't race for the pointer on reconnects
		t := time.NewTicker(1 * time.Second)
		errCount := 0
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			select {
			case <-t.C:
				ok, _, err := client.SendRequest("enos@hashicorp.io", true, nil)
				if !ok {
					errCount++
				}

				if errCount == 5 {
					c.keepaliveErrC <- err
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}

	go startConnectionKeepalive(keepaliveCtx, c.client)

	return nil
}

func (c *client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

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

	if c.client != nil {
		err = c.client.Close()
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

func (c *client) newSession(ctx context.Context) (*xssh.Session, func() error, error) {
	var err error
	var session *xssh.Session
	var cleanup func() error

	wrapErr := func(err error, msg string) error {
		return fmt.Errorf("%s: %w", msg, err)
	}

	if c.client == nil {
		err = c.Connect(ctx)
		if err != nil {
			return session, cleanup, wrapErr(err, "creating client connection")
		}
	}

	session, err = c.client.NewSession()
	if err != nil {
		return session, cleanup, wrapErr(err, "creating SSH session")
	}

	cleanup = func() error {
		err := session.Close()
		if errors.Is(err, io.EOF) {
			return nil
		}

		return err
	}

	// ensure that the session is accepting requests
	requestTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	requestTicker := time.NewTicker(1 * time.Second)
	requestErrs := make(chan error, 7)
	requestSuccess := make(chan bool)

	sendRequest := func() {
		_, err = session.SendRequest("enos@hashicorp.io", true, nil)
		if err == nil {
			requestSuccess <- true
			return
		}
		requestErrs <- fmt.Errorf("sending test SSH session request %w", err)
	}

	drainErrors := func() error {
		merr := &multierror.Error{}
		merr = multierror.Append(merr, err)

		for {
			select {
			case err = <-requestErrs:
				merr = multierror.Append(merr, err)
			default:
				return merr.ErrorOrNil()
			}
		}
	}

	go sendRequest()

	for {
		// Always make sure we haven't hit our timeouts before we attempt another
		// dial.
		select {
		case <-ctx.Done():
			return session, cleanup, wrapErr(drainErrors(), ctx.Err().Error())
		case <-requestTimeout.Done():
			return session, cleanup, wrapErr(drainErrors(), "request timeout exceeded")
		default:
		}

		select {
		case <-ctx.Done():
			return session, cleanup, wrapErr(drainErrors(), ctx.Err().Error())
		case <-requestTimeout.Done():
			return session, cleanup, wrapErr(drainErrors(), "request timeout exceeded")
		case <-requestTicker.C:
			go sendRequest()
		case <-requestSuccess:
			return session, cleanup, nil
		}
	}
}

func (c *client) connectSSHAgent(ctx context.Context) (net.Conn, xssh.AuthMethod, bool) {
	var auth xssh.AuthMethod

	dialer := net.Dialer{}
	sshAgent, err := dialer.DialContext(ctx, "unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return sshAgent, auth, false
	}

	return sshAgent, xssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers), true
}
