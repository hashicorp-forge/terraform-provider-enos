// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ssh

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	it "github.com/hashicorp/enos-provider/internal/transport"

	"github.com/hashicorp/enos-provider/internal/transport/command"
	"github.com/hashicorp/enos-provider/internal/transport/file"
)

// TestSSH tests the SSH transport.
func TestSSH(t *testing.T) {
	t.Parallel()

	// Only performs the test if the environment variables are set
	host, ok := os.LookupEnv("ENOS_TRANSPORT_HOST")
	if !ok {
		t.Skip("SSH tests are skipped unless ENOS_TRANSPORT_* environment variables are set")
	}

	c, err := New(
		WithUser(os.Getenv("ENOS_TRANSPORT_USER")),
		WithHost(host),
		WithKeyPath(os.Getenv("ENOS_TRANSPORT_PRIVATE_KEY_PATH")),
		WithPassphrasePath(os.Getenv("ENOS_TRANSPORT_PASSPHRASE_PATH")),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, c.Close())
	})

	t.Run("copy", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "ssh_test_copy")
		f, err := os.Create(path)
		require.NoError(t, err)
		defer f.Close()

		_, err = f.WriteString("new content")
		require.NoError(t, err)

		src, err := file.Open(path)
		require.NoError(t, err)
		defer src.Close()

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
		defer cancel()

		err = c.Copy(ctx, src, "/tmp/ssh_test_copy")
		require.NoError(t, err)

		_, _, err = c.Run(ctx, command.New("rm /tmp/ssh_test_copy"))
		require.NoError(t, err)
	})

	t.Run("run", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
		defer cancel()

		_, _, err := c.Run(ctx, command.New("printf 'content' > /tmp/ssh_test_run"))
		require.NoError(t, err)

		stdout, _, err := c.Run(ctx, command.New("cat /tmp/ssh_test_run"))
		require.NoError(t, err)
		require.Equal(t, "content", stdout)

		_, _, err = c.Run(ctx, command.New("rm /tmp/ssh_test_run"))
		require.NoError(t, err)

		_, _, err = c.Run(ctx, command.New("cat /tmp/ssh_test_run"))
		require.Error(t, err)
	})

	t.Run("run_exit_1", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
		defer cancel()

		_, _, err := c.Run(ctx, command.New("printf 'exit 1' > /tmp/run_exit_1; chmod +x /tmp/run_exit_1; /tmp/run_exit_1"))
		var exitErr *it.ExecError
		require.ErrorAs(t, err, &exitErr)
		assert.Equal(t, 1, exitErr.ExitCode())

		_, _, err = c.Run(ctx, command.New("rm /tmp/run_exit_1"))
		require.NoError(t, err)
	})

	t.Run("nohup", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(15*time.Second))
		defer cancel()

		// Make sure we can nohup and end our session
		_, _, err := c.Run(ctx, command.New("nohup sleep 7 &>/dev/null &"))
		require.NoError(t, err)
	})
}
