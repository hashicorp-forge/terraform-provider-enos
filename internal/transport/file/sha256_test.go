package file

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSHA256(t *testing.T) {
	sum, err := SHA256("./fixtures/sha256.txt")
	require.NoError(t, err)
	require.Equal(t, "05ab25331487b91eee52f025e7b7f4c09dce324863d7934f057edf43cd87c587", sum)
}
