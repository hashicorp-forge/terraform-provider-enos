package remoteflight

import (
	"context"
	"fmt"
	"strings"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// User is a user.
type User struct {
	Name    string
	HomeDir string
	Shell   string
	GID     string
	UID     string
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
		u.Name = name
		return u
	}
}

// WithUserHomeDir sets the home dir.
func WithUserHomeDir(dir string) UserOpt {
	return func(u *User) *User {
		u.HomeDir = dir
		return u
	}
}

// WithUserShell sets the user shell.
func WithUserShell(shell string) UserOpt {
	return func(u *User) *User {
		u.Shell = shell
		return u
	}
}

// WithUserGID sets the user gid.
func WithUserGID(gid string) UserOpt {
	return func(u *User) *User {
		u.GID = gid
		return u
	}
}

// WithUserUID sets the user uid.
func WithUserUID(uid string) UserOpt {
	return func(u *User) *User {
		u.UID = uid
		return u
	}
}

// FindUser attempts to find details about a user on a remote machine. Currently
// we try to use tools that most-likely exist on most macOS and linux distro's.
// NOTE: If requiring these tools becomes too much of a burden we can create a "user"
// subcommand in flightcontrol to get the details with osusergo.
func FindUser(ctx context.Context, tr it.Transport, name string) (*User, error) {
	var err error
	var stderr string
	user := &User{}

	if name == "" {
		return user, fmt.Errorf("invalid user: you must supply a username")
	}

	user.Name = name

	user.UID, stderr, err = tr.Run(ctx, command.New(fmt.Sprintf("id -u %s", name)))
	if err != nil {
		return user, WrapErrorWith(err, fmt.Sprintf("attempting to get %s uid", name), stderr)
	}

	user.GID, stderr, err = tr.Run(ctx, command.New(fmt.Sprintf("id -g %s", name)))
	if err != nil {
		return user, WrapErrorWith(err, fmt.Sprintf("attempting to get %s gid", name), stderr)
	}

	return user, nil
}

// CreateUser takes a context, transport, and user and creates the user on the
// remote machine.
func CreateUser(ctx context.Context, tr it.Transport, user *User) error {
	if user.Name == "" {
		return fmt.Errorf("invalid user: you must supply a username")
	}

	cmd := strings.Builder{}
	cmd.WriteString("sudo useradd -m --system")
	if user.HomeDir != "" {
		cmd.WriteString(fmt.Sprintf(" --home %s", user.HomeDir))
	}
	if user.Shell != "" {
		cmd.WriteString(fmt.Sprintf(" --shell %s", user.Shell))
	}
	if user.UID != "" {
		cmd.WriteString(fmt.Sprintf(" --uid %s", user.UID))
	}
	if user.GID != "" {
		cmd.WriteString(fmt.Sprintf(" --gid %s", user.GID))
	} else {
		cmd.WriteString(" -U")
	}
	cmd.WriteString(fmt.Sprintf(" %s", user.Name))

	stdout, stderr, err := tr.Run(ctx, command.New(cmd.String()))
	if err != nil {
		return WrapErrorWith(err, stderr, stdout)
	}

	return nil
}

// FindOrCreateUser take a context, transport, and user and attempts to lookup
// the user by the user name. If it fails it will attempt to create the user.
// If the user create succeeds it will lookup the user again and return it WithDownloadRequestUseSudo
// the GID and UID fields populated.
func FindOrCreateUser(ctx context.Context, tr it.Transport, user *User) (*User, error) {
	user, err := FindUser(ctx, tr, user.Name)
	if err == nil {
		return user, nil
	}

	err = CreateUser(ctx, tr, user)
	if err != nil {
		return user, err
	}

	return FindUser(ctx, tr, user.Name)
}
