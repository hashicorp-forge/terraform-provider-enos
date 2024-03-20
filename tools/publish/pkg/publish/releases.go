// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package publish

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// NewRelease returns a new Release.
func NewRelease() *Release {
	return &Release{
		mu:       sync.Mutex{},
		Archives: map[string]*Archive{},
	}
}

// Release is a version collection of archives.
type Release struct {
	mu       sync.Mutex
	Archives map[string]*Archive `json:"archives"` // key is the platform_arch
}

// Archive is a zip archive of a binary.
type Archive struct {
	Hashes    []string `json:"hashes"` // the hash of the zip file
	URL       string   `json:"url"`    // path to the zipfile relative to root
	SHA256Sum string   // The s3 mirror doesn't need this so there's no JSON tag
}

// AddArchive takes a platform, arch and archive and adds it to the releases
// archives.
func (r *Release) AddArchive(platform, arch string, archive *Archive) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Archives[fmt.Sprintf("%s_%s", platform, arch)] = archive
}

// AsJSON writes the release as JSON to the io.Writer.
func (r *Release) AsJSON(to io.Writer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	jsonb, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	reader := bytes.NewReader(jsonb)
	_, err = reader.WriteTo(to)

	return err
}
