package file

import (
	"bytes"
	"crypto/sha256"
	"fmt"
)

// SHA256 takes a file path and returns the SHA256 sum
func SHA256(path string) (string, error) {
	src, err := Open(path)
	if err != nil {
		return "", err
	}

	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(src)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha256.Sum256(buf.Bytes())), src.Close()
}
