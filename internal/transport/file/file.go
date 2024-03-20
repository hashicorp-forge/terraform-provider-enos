// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package file

import (
	"os"
	"time"

	it "github.com/hashicorp/enos-provider/internal/transport"
)

type copyableFile struct {
	info   os.FileInfo
	handle *os.File
}

var _ it.Copyable = (*copyableFile)(nil)

// Open takes a path and return a Copyable.
func Open(path string) (it.Copyable, error) {
	var err error
	f := &copyableFile{}

	f.handle, err = os.Open(path)
	if err != nil {
		return f, err
	}

	f.info, err = f.handle.Stat()

	return f, err
}

func (f *copyableFile) Name() string {
	return f.info.Name()
}

func (f *copyableFile) Size() int64 {
	return f.info.Size()
}

func (f *copyableFile) Mode() os.FileMode {
	return f.info.Mode()
}

func (f *copyableFile) ModTime() time.Time {
	return f.info.ModTime()
}

func (f *copyableFile) IsDir() bool {
	return f.info.IsDir()
}

func (f *copyableFile) Sys() interface{} {
	return f.info.Sys()
}

func (f *copyableFile) Read(until []byte) (int, error) {
	return f.handle.Read(until)
}

func (f *copyableFile) Close() error {
	return f.handle.Close()
}

func (f *copyableFile) Seek(offset int64, whence int) (int64, error) {
	return f.handle.Seek(offset, whence)
}
