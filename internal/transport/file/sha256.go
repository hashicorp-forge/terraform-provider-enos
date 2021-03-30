package file

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
)

// SHA256 takes a file path and returns the SHA256 sum
func SHA256(src io.Reader) (string, error) {
	var err error
	buf := bytes.Buffer{}

	_, err = buf.ReadFrom(src)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha256.Sum256(buf.Bytes())), nil
}
