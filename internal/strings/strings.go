// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package strings

import stdstrings "strings"

// Indent takes a prefix string and a body string and prefixes every newline
// in the body string with the prefix string.
func Indent(indent string, s string) string {
	if indent == "" || s == "" {
		return s
	}

	lines := stdstrings.SplitAfter(s, "\n")
	if len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}

	return stdstrings.Join(append([]string{""}, lines...), indent)
}
