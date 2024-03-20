// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package publish

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
)

// NewIndex returns a new Index.
func NewIndex() *Index {
	return &Index{
		mu:       sync.Mutex{},
		Versions: map[string]*IndexValue{},
	}
}

// Index is our root index.json which tracks which versions are available in the
// mirror.
type Index struct {
	mu       sync.Mutex
	Versions map[string]*IndexValue `json:"versions"`
}

// AsJSON takes an io.Writes and writes itself as JSON to it.
func (i *Index) AsJSON(to io.Writer) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	jsonb, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}

	reader := bytes.NewReader(jsonb)
	_, err = reader.WriteTo(to)

	return err
}

// IndexValue is the value of Version in the Index. Currently it is a blank
// object.
type IndexValue struct{}
