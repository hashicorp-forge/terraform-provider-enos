// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
)

// execConfig User provided configuration for an exec Resource.
type execConfig struct {
	Env     *tfStringMap
	Content *tfString
	Inline  *tfStringSlice
	Scripts *tfStringSlice
}

// computeSHA256 Computes the SHA256 sum for an execConfig.
func (e execConfig) computeSHA256(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// We're probably overthinking this but this is a sha256 sum of the
	// aggregate of the inline commands, the rendered content, and scripts.
	ag := strings.Builder{}

	if cont, ok := e.Content.Get(); ok {
		content := tfile.NewReader(cont)
		defer content.Close()

		sha, err := tfile.SHA256(content)
		if err != nil {
			return "", AttributePathError(
				fmt.Errorf("invalid configuration, unable to determine content SHA256 sum, due to: %w", err),
				"content",
			)
		}

		ag.WriteString(sha)
	}

	if inline, ok := e.Inline.GetStrings(); ok {
		for _, cmd := range inline {
			ag.WriteString(command.SHA256(command.New(cmd)))
		}
	}

	if scripts, ok := e.Scripts.GetStrings(); ok {
		var sha string
		var file it.Copyable
		var err error
		for _, path := range scripts {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
			}

			file, err = tfile.Open(path)
			if err != nil {
				return "", AttributePathError(
					fmt.Errorf("invalid configuration, unable to open scripts file: [%s], due to: %w", path, err),
					"scripts",
				)
			}
			defer file.Close()

			sha, err = tfile.SHA256(file)
			if err != nil {
				return "", AttributePathError(
					fmt.Errorf("invalid configuration, unable to determine script file SHA256 sum, due to: %w", err),
					"scripts",
				)
			}

			ag.WriteString(sha)
		}
	}

	if env, ok := e.Env.GetStrings(); ok && len(env) > 0 {
		var keys []string
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, key := range keys {
			b := []byte(fmt.Sprintf("%s:%s", key, env[key]))
			sha := fmt.Sprintf("%x", sha256.Sum256(b))
			ag.WriteString(sha)
		}
	}

	return fmt.Sprintf("%x", sha256.Sum256([]byte(ag.String()))), nil
}
