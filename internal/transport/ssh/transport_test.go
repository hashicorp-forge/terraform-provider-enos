package ssh

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/enos-provider/internal/transport/command"
	"github.com/hashicorp/enos-provider/internal/transport/file"
)

// TestSSH tests the SSH transport
func TestSSH(t *testing.T) {
	// Only performs the test if the environment variables are set
	host, ok := os.LookupEnv("ENOS_TRANSPORT_HOST")
	if !ok {
		t.Skip("SSH tests are skipped unless ENOS_TRANSPORT_* environment variables are set")
	}

	c, err := New(
		WithUser(os.Getenv("ENOS_TRANSPORT_USER")),
		WithHost(host),
		WithKeyPath(os.Getenv("ENOS_TRANSPORT_KEY_PATH")),
		WithPassphrasePath(os.Getenv("ENOS_TRANSPORT_PASSPHRASE_PATH")),
	)
	require.NoError(t, err)

	t.Run("copy", func(t *testing.T) {
		f, err := os.Create("/tmp/ssh_test")
		require.NoError(t, err)
		defer os.Remove("/tmp/ssh_test")

		_, err = f.WriteString("new content")
		require.NoError(t, err)

		src, err := file.Open("/tmp/ssh_test")
		require.NoError(t, err)

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
		defer cancel()

		err = c.Copy(ctx, src, "/tmp/ssh_test")
		require.NoError(t, err)

		_, _, err = c.Run(ctx, command.New("rm /tmp/ssh_test"))
		require.NoError(t, err)

		require.NoError(t, c.Close())
	})

	t.Run("run", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
		defer cancel()

		_, _, err := c.Run(ctx, command.New("printf 'content' > /tmp/run"))
		require.NoError(t, err)

		stdout, _, err := c.Run(ctx, command.New("cat /tmp/run"))
		require.NoError(t, err)
		require.Equal(t, "content", stdout)

		_, _, err = c.Run(ctx, command.New("rm /tmp/run"))
		require.NoError(t, err)

		_, _, err = c.Run(ctx, command.New("cat /tmp/run"))
		require.Error(t, err)

		require.NoError(t, c.Close())
	})
}
