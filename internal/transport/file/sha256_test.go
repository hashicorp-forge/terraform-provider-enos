package file

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSHA256(t *testing.T) {
	t.Run("with file", func(t *testing.T) {
		f, err := Open("./fixtures/sha256.txt")
		require.NoError(t, err)

		sum, err := SHA256(f)
		require.NoError(t, err)
		require.Equal(t, "05ab25331487b91eee52f025e7b7f4c09dce324863d7934f057edf43cd87c587", sum)

		require.NoError(t, f.Close())
	})

	t.Run("with string", func(t *testing.T) {
		r := NewReader("sha256 content\n")

		sum, err := SHA256(r)
		require.NoError(t, err)
		require.Equal(t, "05ab25331487b91eee52f025e7b7f4c09dce324863d7934f057edf43cd87c587", sum)

		require.NoError(t, r.Close())
	})
}
