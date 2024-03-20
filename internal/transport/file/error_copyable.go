// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package file

import (
	"errors"

	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
)

// errorCopyable implement Copyable and throws an error on read and seek methods.
type errorCopyable struct {
	err error
}

func NewErrorCopyable(errorMessage string) it.Copyable {
	return &errorCopyable{errors.New(errorMessage)}
}

func (e errorCopyable) Read(p []byte) (n int, err error) {
	return -1, e.err
}

func (e errorCopyable) Seek(offset int64, whence int) (int64, error) {
	return -1, e.err
}

func (e errorCopyable) Close() error {
	return nil
}

func (e errorCopyable) Size() int64 {
	return 10
}
