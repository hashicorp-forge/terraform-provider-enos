package systemd

import (
	"fmt"
	"strings"
)

// Unit is a map structure representing any systemd unit. The first keys
// represent stanzas and the the values of each stanza is a map of filed names
// and values.
type Unit map[string]map[string]string

// Iniable is an interface for a type that can be converted into a systemd unit
type Iniable interface {
	ToIni() (string, error)
}

var _ Iniable = (Unit)(nil)

// ToIni converts a Unit to the textual representation.  Due to go maps
// not being ordered, the Unit may render differently each time that the function
// is called. In testing this hasn't shown any negative effects but might be
// confusing.
func (s Unit) ToIni() (string, error) {
	unit := &strings.Builder{}

	for stanza, fields := range s {
		if len(fields) == 0 {
			continue
		}

		unit.WriteString(fmt.Sprintf("[%s]\n", stanza))
		for k, v := range fields {
			unit.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}

		unit.WriteString("\n")
	}

	return strings.TrimSpace(unit.String()), nil
}

// CreateUnitFileRequest is a systemd unit file creator
type CreateUnitFileRequest struct {
	Unit     Iniable
	UnitPath string
	Chmod    string
	Chown    string
}

// CreateUnitFileOpt is a functional option for an systemd unit request
type CreateUnitFileOpt func(*CreateUnitFileRequest) *CreateUnitFileRequest

// NewCreateUnitFileRequest takes functional options and returns a new
// systemd unit request
func NewCreateUnitFileRequest(opts ...CreateUnitFileOpt) *CreateUnitFileRequest {
	c := &CreateUnitFileRequest{}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithUnitUnitPath sets the unit name
func WithUnitUnitPath(path string) CreateUnitFileOpt {
	return func(u *CreateUnitFileRequest) *CreateUnitFileRequest {
		u.UnitPath = path
		return u
	}
}

// WithUnitFile sets systemd unit to use
func WithUnitFile(unit Iniable) CreateUnitFileOpt {
	return func(u *CreateUnitFileRequest) *CreateUnitFileRequest {
		u.Unit = unit
		return u
	}
}

// WithUnitChmod sets systemd unit permissions
func WithUnitChmod(chmod string) CreateUnitFileOpt {
	return func(u *CreateUnitFileRequest) *CreateUnitFileRequest {
		u.Chmod = chmod
		return u
	}
}

// WithUnitChown sets systemd unit ownership
func WithUnitChown(chown string) CreateUnitFileOpt {
	return func(u *CreateUnitFileRequest) *CreateUnitFileRequest {
		u.Chown = chown
		return u
	}
}
