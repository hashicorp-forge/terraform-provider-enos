package remoteflight

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// User is a system user.
type User struct {
	Name    *string
	HomeDir *string
	Shell   *string
	GID     *string
	UID     *string
}

// UserOpt is a functional option for an user request.
type UserOpt func(*User) *User

// NewUser takes functional options and returns a new user.
func NewUser(opts ...UserOpt) *User {
	u := &User{}

	for _, opt := range opts {
		u = opt(u)
	}

	return u
}

// WithUserName sets the user name.
func WithUserName(name string) UserOpt {
	return func(u *User) *User {
		u.Name = &name
		return u
	}
}

// WithUserHomeDir sets the home dir.
func WithUserHomeDir(dir string) UserOpt {
	return func(u *User) *User {
		u.HomeDir = &dir
		return u
	}
}

// WithUserShell sets the user shell.
func WithUserShell(shell string) UserOpt {
	return func(u *User) *User {
		u.Shell = &shell
		return u
	}
}

// WithUserGID sets the user gid.
func WithUserGID(gid string) UserOpt {
	return func(u *User) *User {
		u.GID = &gid
		return u
	}
}

// WithUserUID sets the user uid.
func WithUserUID(uid string) UserOpt {
	return func(u *User) *User {
		u.UID = &uid
		return u
	}
}

// FindUser attempts to find details about a user on a remote machine. Currently
// we try to use tools that most-likely exist on most macOS and linux distro's.
func FindUser(ctx context.Context, tr it.Transport, name string) (*User, error) {
	// Try getent as it will decode to our line for us
	user, err := findUserGetEnt(ctx, tr, name)
	if err == nil {
		return user, nil
	}

	// Try reading /etc/passwd ourselves
	user, err1 := findUserPasswd(ctx, tr, name)
	err = errors.Join(err, err1)
	if err1 == nil {
		return user, nil
	}

	// Fallback to id get to UID and GID. We won't get home dir or shell with
	user, err1 = findUserID(ctx, tr, name)
	err = errors.Join(err, err1)
	if err1 == nil {
		return user, nil
	}

	return nil, err
}

// CreateUser takes a context, transport, and user and creates the user on the
// remote machine.
func CreateUser(ctx context.Context, tr it.Transport, user *User) (*User, error) {
	if user == nil || user.Name == nil {
		return nil, fmt.Errorf("invalid user: you must supply a username")
	}

	cmd := strings.Builder{}
	cmd.WriteString("sudo useradd -m --system")
	if user.HomeDir != nil {
		cmd.WriteString(fmt.Sprintf(" --home %s", *user.HomeDir))
	}
	if user.Shell != nil {
		cmd.WriteString(fmt.Sprintf(" --shell %s", *user.Shell))
	}
	if user.UID != nil {
		cmd.WriteString(fmt.Sprintf(" --uid %s", *user.UID))
	}
	if user.GID != nil {
		cmd.WriteString(fmt.Sprintf(" --gid %s", *user.GID))
	} else {
		cmd.WriteString(" -U")
	}
	cmd.WriteString(fmt.Sprintf(" %s", *user.Name))

	stdout, stderr, err := tr.Run(ctx, command.New(cmd.String()))
	if err != nil {
		return nil, WrapErrorWith(err, stderr, stdout)
	}

	return FindUser(ctx, tr, *user.Name)
}

// CreateOrUpdateUser takes an wanted user specification and creates or updates a user to the
// specification.
func CreateOrUpdateUser(ctx context.Context, tr it.Transport, want *User) (*User, error) {
	if want == nil || want.Name == nil {
		return nil, fmt.Errorf("update user: invalid update specification")
	}

	have, err := FindUser(ctx, tr, *want.Name)
	if err != nil {
		// We can't find a user with that name. Create one.
		return CreateUser(ctx, tr, want)
	}

	if want.HasSameSetProperties(have) {
		// We have nothing to update.
		return have, nil
	}

	// Any of our HomeDir, Shell, UID, or GID could be mismatched. Check them all and update them
	// if necessary.
	if want.HomeDir != nil && want.HomeDir != have.HomeDir {
		stdout, stderr, err := tr.Run(ctx, command.New(fmt.Sprintf("sudo usermod -d %s %s", *want.HomeDir, *want.Name)))
		if err != nil {
			return nil, WrapErrorWith(err, stderr, stdout)
		}
	}

	if want.Shell != nil && want.Shell != have.Shell {
		stdout, stderr, err := tr.Run(ctx, command.New(fmt.Sprintf("sudo usermod -s %s %s", *want.Shell, *want.Name)))
		if err != nil {
			return nil, WrapErrorWith(err, stderr, stdout)
		}
	}

	if want.UID != nil && want.UID != have.UID {
		stdout, stderr, err := tr.Run(ctx, command.New(fmt.Sprintf("sudo usermod -u %s %s", *want.UID, *want.Name)))
		if err != nil {
			return nil, WrapErrorWith(err, stderr, stdout)
		}
	}

	if want.GID != nil && want.GID != have.GID {
		stdout, stderr, err := tr.Run(ctx, command.New(fmt.Sprintf("sudo usermod -g %s %s", *want.GID, *want.Name)))
		if err != nil {
			return nil, WrapErrorWith(err, stderr, stdout)
		}
	}

	return FindUser(ctx, tr, *want.Name)
}

// HasSameSetProperties takes a user specification and verifies that the current user has the same
// set properties. The zero values will not be considered.
func (u *User) HasSameSetProperties(other *User) bool {
	if other == nil {
		return true
	}

	if u == nil {
		return false
	}

	if other.Name != nil {
		if other.Name != u.Name {
			return false
		}
	}

	if other.HomeDir != nil {
		if other.HomeDir != u.HomeDir {
			return false
		}
	}

	if other.Shell != nil {
		if other.Shell != u.Shell {
			return false
		}
	}

	if other.GID != nil {
		if other.GID != u.GID {
			return false
		}
	}

	if other.UID != nil {
		if other.UID != u.UID {
			return false
		}
	}

	return true
}

func decodePasswdLine(line string) (*User, error) {
	if line == "" {
		return nil, fmt.Errorf("cannot decode blank /etc/passwd line")
	}

	user := &User{}
	parts := strings.Split(line, ":")
	if len(parts) < 7 {
		return nil, fmt.Errorf("malformed /etc/passwd entry: expected 7 fields, got %d", len(parts))
	}
	user.Name = &parts[0]
	// parts[1] is the "password", but not really since we crypt that
	user.UID = &parts[2]
	user.GID = &parts[3]
	// parts[4] is the GECOS, which may contain the full name, but we don't care about that.
	user.HomeDir = &parts[5]
	user.Shell = &parts[6]

	return user, nil
}

func findUserGetEnt(ctx context.Context, tr it.Transport, name string) (*User, error) {
	// Try getent first
	line, stderr, err := tr.Run(ctx, command.New(fmt.Sprintf("getent passwd %s", name)))
	if err != nil {
		return nil, fmt.Errorf("finding user %s, getent passwd %s: %w: %s", name, name, err, stderr)
	}

	return decodePasswdLine(line)
}

func findUserPasswd(ctx context.Context, tr it.Transport, name string) (*User, error) {
	passwd, stderr, err := tr.Run(ctx, command.New("/etc/passwd"))
	if err != nil {
		return nil, fmt.Errorf("finding user %s, attempting to read contents of /etc/passwd: %w: %s", name, err, stderr)
	}

	scanner := bufio.NewScanner(strings.NewReader(passwd))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, name) {
			return decodePasswdLine(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading /etc/password: %w", err)
	}

	return nil, fmt.Errorf("could not find user %s", name)
}

func findUserID(ctx context.Context, tr it.Transport, name string) (*User, error) {
	user := &User{Name: &name}
	var err error
	var stderr string

	uid, stderr, err := tr.Run(ctx, command.New(fmt.Sprintf("id -u %s", name)))
	if err != nil {
		return user, WrapErrorWith(err, fmt.Sprintf("attempting to get %s uid", name), stderr)
	}
	user.UID = &uid

	gid, stderr, err := tr.Run(ctx, command.New(fmt.Sprintf("id -g %s", name)))
	if err != nil {
		return user, WrapErrorWith(err, fmt.Sprintf("attempting to get %s gid", name), stderr)
	}
	user.GID = &gid

	return user, nil
}
