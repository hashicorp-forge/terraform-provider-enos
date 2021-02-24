package plugin

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

func readTestFile(path string) (string, error) {
	res := ""
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
