// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"crypto/sha256"
	"fmt"

	it "github.com/hashicorp/enos-provider/internal/transport"
)

// SHA256 returns the SHA256 sum of the command.
func SHA256(cmd it.Command) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(cmd.Cmd())))
}
