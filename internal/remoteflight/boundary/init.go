package boundary

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	it "github.com/hashicorp/enos-provider/internal/transport"
	"github.com/hashicorp/enos-provider/internal/transport/command"
)

// InitRequest is a Boundary init request
type InitRequest struct {
	*CLIRequest
}

// InitResponse is the Boundary init response
type InitResponse struct {
	AuthMethod    AuthMethod    `json:"auth_method"`
	HostResources HostResources `json:"host_resources"`
	LoginRole     LoginRole     `json:"login_role"`
	OrgScope      OrgScope      `json:"org_scope"`
	ProjectScope  ProjectScope  `json:"project_scope"`
	Target        Target        `json:"target"`
}

// AuthMethod is the Boundary init generated authentication method
type AuthMethod struct {
	AuthMethodID   string `json:"auth_method_id"`
	AuthMethodName string `json:"auth_method_name"`
	LoginName      string `json:"login_name"`
	Password       string `json:"password"`
	ScopeID        string `json:"scope_id"`
	UserID         string `json:"user_id"`
	UserName       string `json:"user_name"`
}

// HostResources are the Boundary init generated host resources
type HostResources struct {
	HostCatalogID   string `json:"host_catalog_id"`
	HostSetID       string `json:"host_set_id"`
	HostID          string `json:"host_id"`
	Type            string `json:"type"`
	ScopeID         string `json:"scope_id"`
	HostCatalogName string `json:"host_catalog_name"`
	HostSetName     string `json:"host_set_name"`
	HostName        string `json:"host_name"`
}

// LoginRole is the Boundary init generated login role
type LoginRole struct {
	ScopeID string `json:"scope_id"`
	Name    string `json:"name"`
}

// OrgScope is the Boundary init generated organization scope
type OrgScope struct {
	ScopeID string `json:"scope_id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
}

// ProjectScope is the Boundary init generated project scope
type ProjectScope struct {
	ScopeID string `json:"scope_id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
}

// Target is the Boundary init generated target
type Target struct {
	TargetID               string `json:"target_id"`
	DefaultPort            int    `json:"default_port"`
	SessionMaxSeconds      int    `json:"session_max_seconds"`
	SessionConnectionLimit int    `json:"session_connection_limit"`
	Type                   string `json:"type"`
	ScopeID                string `json:"scope_id"`
	Name                   string `json:"name"`
}

// InitRequestOpt is a functional option for an init request
type (
	InitRequestOpt func(*InitRequest) *InitRequest
)

// NewInitRequest takes functional options and returns a new
// init request
func NewInitRequest(opts ...InitRequestOpt) *InitRequest {
	c := &InitRequest{
		&CLIRequest{},
	}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithInitRequestBinName sets the Boundary binary name
func WithInitRequestBinName(name string) InitRequestOpt {
	return func(i *InitRequest) *InitRequest {
		i.BinName = name
		return i
	}
}

// WithInitRequestBinPath sets the Boundary binary path
func WithInitRequestBinPath(path string) InitRequestOpt {
	return func(i *InitRequest) *InitRequest {
		i.BinPath = path
		return i
	}
}

// WithInitRequestConfigPath sets the Boundary config path
func WithInitRequestConfigPath(path string) InitRequestOpt {
	return func(i *InitRequest) *InitRequest {
		i.ConfigPath = path
		return i
	}
}

func WithInitRequestLicense(license string) InitRequestOpt {
	return func(i *InitRequest) *InitRequest {
		i.License = license
		return i
	}
}

// Validate validates that the init request has the required fields
func (r *InitRequest) Validate() error {
	if r.BinPath == "" {
		return fmt.Errorf("no binary path has been supplied")
	}
	if r.ConfigPath == "" {
		return fmt.Errorf("no config path has been supplied")
	}
	// License is optional for OSS, required for Enterprise
	return nil
}

// String returns the init request as an init command
func (r *InitRequest) String() string {
	cmd := &strings.Builder{}
	cmd.WriteString(fmt.Sprintf("%s/%s database init -format json", r.BinPath, r.BinName))
	// TODO: add the other init options for upgrades
	cmd.WriteString(fmt.Sprintf(" -config=%s/boundary.hcl", r.ConfigPath))
	return cmd.String()
}

// Init calls boundary init to initialize the database and Boundary cluster,
// providing default credentials to use upon completion
func Init(ctx context.Context, ssh it.Transport, req *InitRequest) (*InitResponse, error) {
	res := &InitResponse{}
	envVars := map[string]string{}

	if req.License != "" {
		envVars["BOUNDARY_LICENSE"] = req.License
	}

	stdout, stderr, err := ssh.Run(ctx, command.New(
		req.String(),
		command.WithEnvVars(envVars)))
	if err != nil {
		return res, fmt.Errorf("failed to init: %s stderr: %s", err, stderr)
	}

	if stdout == "" {
		return res, fmt.Errorf("boundary init failed to return output")
	}

	err = json.Unmarshal([]byte(stdout), &res)
	if err != nil {
		return res, fmt.Errorf("failed to unmarshal json: %s, stdout: %q", err, stdout)
	}

	return res, nil
}
