// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package file

import (
	"io"
	"strings"

	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
)

type stringCopyable struct {
	reader *strings.Reader
}

var _ it.Copyable = (*stringCopyable)(nil)

func NewReader(content string) it.Copyable {
	s := &stringCopyable{}
	s.reader = strings.NewReader(content)

	return s
}

func (s *stringCopyable) Len() int {
	return s.reader.Len()
}

func (s *stringCopyable) Read(b []byte) (int, error) {
	return s.reader.Read(b)
}

func (s *stringCopyable) ReadAt(b []byte, off int64) (int, error) {
	return s.reader.ReadAt(b, off)
}

func (s *stringCopyable) ReadByte() (byte, error) {
	return s.reader.ReadByte()
}

func (s *stringCopyable) ReadRune() (rune, int, error) {
	return s.reader.ReadRune()
}

func (s *stringCopyable) Reset(str string) {
	s.reader.Reset(str)
}

func (s *stringCopyable) Seek(offset int64, whence int) (int64, error) {
	return s.reader.Seek(offset, whence)
}

func (s *stringCopyable) Size() int64 {
	return s.reader.Size()
}

func (s *stringCopyable) UnreadByte() error {
	return s.reader.UnreadByte()
}

func (s *stringCopyable) UnreadRune() error {
	return s.reader.UnreadRune()
}

func (s *stringCopyable) WriteTo(w io.Writer) (int64, error) {
	return s.reader.WriteTo(w)
}

func (s *stringCopyable) Close() error {
	s.reader.Reset("")
	return nil
}
