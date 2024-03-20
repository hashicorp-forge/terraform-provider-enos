// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package random

import (
	"math/rand"
	"os"
	"strconv"
	"time"
)

// ID returns a random ID.
func ID() string {
	//nolint: gosec
	r := rand.New(rand.NewSource(time.Now().UnixNano() * int64(os.Getpid())))

	return strconv.FormatInt(int64(r.Int31()), 10)
}
