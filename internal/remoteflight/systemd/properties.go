// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package systemd

import (
	"errors"
	"fmt"
	"strings"
)

// UnitProperties are a key value map of unit properties.
type UnitProperties map[string]string

// EnabledAndRunningProperties are properties that a systemd service unit
// should have to be considered active and running.
var EnabledAndRunningProperties = UnitProperties{
	"LoadState":     "loaded",
	"ActiveState":   "active",
	"SubState":      "running",
	"UnitFileState": "enabled",
}

// NewUnitProperties returns a new NewUnitProperties.
func NewUnitProperties() UnitProperties {
	return UnitProperties{}
}

// HasProperties determines whether the current unit properties contains all
// properties of another property set. This does not enfore exact comparison,
// only that the first set shares all from the second set.
func (s UnitProperties) HasProperties(in UnitProperties) bool {
	for prop, val := range in {
		sval, ok := s[prop]
		if !ok || val != sval {
			return false
		}
	}

	return true
}

// Find takes one-or-more property names and returns a set of unit properties
// that match the names. Unless all properties are found an error will be
// returned.
func (s UnitProperties) Find(names ...string) (UnitProperties, error) {
	var err error
	props := NewUnitProperties()

	for _, name := range names {
		val, ok := s[name]
		if !ok {
			err = errors.Join(err, fmt.Errorf("no property found: %s", name))
		}
		props[name] = val
	}

	return props, err
}

// FindProperties is like Find() but instead of a list of values it derives the
// key name from those defined in the pass in UnitProperties. The values of
// properties in the given set are ignored.
func (s UnitProperties) FindProperties(in UnitProperties) (UnitProperties, error) {
	names := make([]string, len(in))

	count := 0
	for name := range in {
		names[count] = name
		count++
	}

	return s.Find(names...)
}

func (s UnitProperties) String() string {
	out := new(strings.Builder)

	for name, value := range s {
		_, _ = out.WriteString(fmt.Sprintf("%s=%s\n", name, value))
	}

	return out.String()
}
