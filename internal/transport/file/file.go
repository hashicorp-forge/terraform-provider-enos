package file

import (
	"os"
	"time"

	it "github.com/hashicorp/enos-provider/internal/transport"
)

type copyable struct {
	info   os.FileInfo
	handle *os.File
}

var _ it.Copyable = (*copyable)(nil)

// Open takes a path and return a Copyable
func Open(path string) (it.Copyable, error) {
	var err error
	f := &copyable{}

	f.handle, err = os.Open(path)
	if err != nil {
		return f, err
	}

	f.info, err = f.handle.Stat()

	return f, err
}

func (f *copyable) Name() string {
	return f.info.Name()
}

func (f *copyable) Size() int64 {
	return f.info.Size()
}

func (f *copyable) Mode() os.FileMode {
	return f.info.Mode()
}

func (f *copyable) ModTime() time.Time {
	return f.info.ModTime()
}

func (f *copyable) IsDir() bool {
	return f.info.IsDir()
}

func (f *copyable) Sys() interface{} {
	return f.info.Sys()
}

func (f *copyable) Read(until []byte) (int, error) {
	return f.handle.Read(until)
}

func (f *copyable) Close() error {
	return f.handle.Close()
}

func (f *copyable) Seek(offset int64, whence int) (int64, error) {
	return f.handle.Seek(offset, whence)
}
