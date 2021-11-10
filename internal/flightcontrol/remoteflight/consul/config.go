package consul

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/hashicorp/enos-provider/internal/flightcontrol/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
)

// HCLConfig is a struct that represents a basic version of the Consul config.
type HCLConfig struct {
	Datacenter      string
	DataDir         string
	RetryJoin       []string
	Server          bool
	BootstrapExpect int
	LogFile         string
	LogLevel        string
}

// HCLBlock is a nested block in the consul config
type HCLBlock struct {
	Label string
	Attrs HCLBlockAttrs
}

// HCLBlockAttrs are block attributes
type HCLBlockAttrs map[string]interface{}

// HCLConfigTemplate the the default configuration template
var HCLConfigTemplate = template.Must(template.New("consul").Parse(`{{if .Datacenter -}}
datacenter = "{{.Datacenter}}"
{{end -}}

{{if .DataDir -}}
data_dir = "{{.DataDir}}"
{{end -}}

{{if .RetryJoin -}}
retry_join = [{{range .RetryJoin -}}
  "{{.}}",
{{end}}]
{{end -}}

{{if .Server -}}
server = {{.Server}}
{{end -}}

{{if .BootstrapExpect -}}
bootstrap_expect = {{.BootstrapExpect}}
{{end -}}

{{if .LogFile -}}
log_file = "{{.LogFile}}"
{{end -}}

{{if .LogLevel -}}
log_level = "{{.LogLevel}}"
{{end -}}
`))

// HCLable is an interface for a type that can be converted into a consul config
type HCLable interface {
	ToHCL() (string, error)
}

var _ HCLable = (*HCLConfig)(nil)

// ToHCL converts a HCLConfig to the textual representation
func (s *HCLConfig) ToHCL() (string, error) {
	buf := bytes.Buffer{}
	err := HCLConfigTemplate.Execute(&buf, s)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// CreateHCLConfigFileRequest is a consul HCL config create request
type CreateHCLConfigFileRequest struct {
	HCLConfig HCLable
	FilePath  string
	Chmod     string
	Chown     string
}

// CreateHCLConfigFileOpt is a functional option for a config create request
type CreateHCLConfigFileOpt func(*CreateHCLConfigFileRequest) *CreateHCLConfigFileRequest

// NewCreateHCLConfigFileRequest takes functional options and returns a new
// Consul config file request
func NewCreateHCLConfigFileRequest(opts ...CreateHCLConfigFileOpt) *CreateHCLConfigFileRequest {
	c := &CreateHCLConfigFileRequest{}

	for _, opt := range opts {
		c = opt(c)
	}

	return c
}

// WithHCLConfigFilePath sets the config file path
func WithHCLConfigFilePath(path string) CreateHCLConfigFileOpt {
	return func(u *CreateHCLConfigFileRequest) *CreateHCLConfigFileRequest {
		u.FilePath = path
		return u
	}
}

// WithHCLConfig sets the config file to use
func WithHCLConfig(unit HCLable) CreateHCLConfigFileOpt {
	return func(u *CreateHCLConfigFileRequest) *CreateHCLConfigFileRequest {
		u.HCLConfig = unit
		return u
	}
}

// WithHCLConfigChmod sets config file permissions
func WithHCLConfigChmod(chmod string) CreateHCLConfigFileOpt {
	return func(u *CreateHCLConfigFileRequest) *CreateHCLConfigFileRequest {
		u.Chmod = chmod
		return u
	}
}

// WithHCLConfigChown sets config file ownership
func WithHCLConfigChown(chown string) CreateHCLConfigFileOpt {
	return func(u *CreateHCLConfigFileRequest) *CreateHCLConfigFileRequest {
		u.Chown = chown
		return u
	}
}

// CreateHCLConfigFile takes a context, transport, and create request and
// creates the config file.
func CreateHCLConfigFile(ctx context.Context, ssh it.Transport, req *CreateHCLConfigFileRequest) error {
	hcl, err := req.HCLConfig.ToHCL()
	if err != nil {
		return fmt.Errorf("marshaling configuration to HCL: %w", err)
	}

	if req.FilePath == "" {
		return fmt.Errorf("you must provide a config file destination path")
	}

	copyOpts := []remoteflight.CopyFileRequestOpt{
		remoteflight.WithCopyFileContent(tfile.NewReader(hcl)),
		remoteflight.WithCopyFileDestination(req.FilePath),
	}

	if req.Chmod != "" {
		copyOpts = append(copyOpts, remoteflight.WithCopyFileChmod(req.Chmod))
	}

	if req.Chown != "" {
		copyOpts = append(copyOpts, remoteflight.WithCopyFileChown(req.Chown))
	}

	return remoteflight.CopyFile(ctx, ssh, remoteflight.NewCopyFileRequest(copyOpts...))
}
