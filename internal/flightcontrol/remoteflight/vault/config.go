package vault

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/hashicorp/enos-provider/internal/flightcontrol/remoteflight"
	it "github.com/hashicorp/enos-provider/internal/transport"
	tfile "github.com/hashicorp/enos-provider/internal/transport/file"
)

// HCLConfig is a struct that represents a basic version of the Vault config.
// As the Vault parser package only supports decoding from HCL into structs
// and not encoding, we either have to take raw strings of HCL or implement
// a basic representation that can encode itself into HCL. Feel free to extend
// this as necessary, or build your own HCLable type that exports the HCL that
// you need. A proper server config encoder would be lovely someday.
type HCLConfig struct {
	APIAddr                   string // ha backend only
	CacheSize                 string
	ClusterAddr               string // ha backend only
	ClusterName               string
	DefaultMaxRequestDuration string
	DefaultLeaseTTL           string
	DisableCache              bool
	DisableClustering         bool // ha backend only
	DisableMlock              bool
	DisablePerformanceStandby bool // enterprise only
	DisableSealwrap           bool // enterprise only
	HAStorage                 *HCLBlock
	Listener                  *HCLBlock
	LogLevel                  string
	MaxLeaseTTL               string
	PidFile                   string
	PluginDirectory           string
	RawStorageEndpoint        bool
	Seal                      *HCLBlock
	Storage                   *HCLBlock
	Telemetry                 *HCLBlock
	UI                        bool
}

// HCLBlock is a nested block in the vault config
type HCLBlock struct {
	Label string
	Attrs HCLBlockAttrs
}

// HCLBlockAttrs are block attributes
type HCLBlockAttrs map[string]interface{}

// HCLConfigTemplate the the default configuration template
var HCLConfigTemplate = template.Must(template.New("vault").Funcs(template.FuncMap{
	"parseBlock": func(btype string, block *HCLBlock) string {
		if block == nil {
			return ""
		}

		out := &strings.Builder{}

		if block.Label != "" {
			out.WriteString(fmt.Sprintf("%s \"%s\" {\n", btype, block.Label))
		} else {
			out.WriteString(fmt.Sprintf("%s {\n", btype))
		}

		if len(block.Attrs) > 0 {
			// Iterated the attrs by sorted key so that attributes are
			// written deterministically, otherwise it's hard to test this.
			keys := []string{}
			for k := range block.Attrs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				switch rv := block.Attrs[k].(type) {
				case nil:
				case int:
					out.WriteString(fmt.Sprintf("  %s = %d\n", k, rv))
				case bool:
					out.WriteString(fmt.Sprintf("  %s = %t\n", k, rv))
				case string:
					out.WriteString(fmt.Sprintf("  %s = \"%s\"\n", k, rv))
				default:
				}
			}
		}
		out.WriteString("}")

		return out.String()
	},
}).Parse(`{{if .APIAddr -}}
api_addr = "{{.APIAddr}}"
{{end -}}

{{if .CacheSize -}}
cache_size = "{{.CacheSize}}"
{{end -}}

{{if .ClusterAddr -}}
cluster_addr = "{{.ClusterAddr}}"
{{end -}}

{{if .ClusterName -}}
cluster_name = "{{.ClusterName}}"
{{end -}}

{{if .DefaultMaxRequestDuration -}}
default_max_request_duration = "{{.DefaultMaxRequestDuration}}"
{{end -}}

{{if .DefaultLeaseTTL -}}
default_lease_ttl = "{{.DefaultLeaseTTL}}"
{{end -}}

{{if .DisableCache -}}
disable_cache = true
{{end -}}

{{if .DisableClustering -}}
disable_clustering = true
{{end -}}

{{if .DisableMlock -}}
disable_mlock = true
{{end -}}

{{if .DisablePerformanceStandby -}}
disable_performance_standby = true
{{end -}}

{{if .DisableSealwrap -}}
disable_sealwrap = true
{{end -}}

{{if .HAStorage -}}
{{parseBlock "ha_storage" .HAStorage }}
{{end -}}

{{if .Listener -}}
{{parseBlock "listener" .Listener }}
{{end -}}

{{if .LogLevel -}}
log_level = "{{.LogLevel}}"
{{end -}}

{{if .MaxLeaseTTL -}}
max_lease_ttl = "{{.MaxLeaseTTL}}"
{{end -}}

{{if .PidFile -}}
pid_file = "{{.PidFile}}"
{{end -}}

{{if .PluginDirectory -}}
PluginDirectory = "{{.PluginDirectory}}"
{{end -}}

{{if .RawStorageEndpoint -}}
raw_storage_endpoint = true
{{end -}}

{{if .Seal -}}
{{parseBlock "seal" .Seal }}
{{end -}}

{{if .Storage -}}
{{parseBlock "storage" .Storage }}
{{end -}}

{{if .Telemetry -}}
{{parseBlock "telemetry" .Telemetry }}
{{end -}}

{{if .UI -}}
ui = true
{{end -}}
`))

// HCLable is an interface for a type that can be converted into a vault config
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

// CreateHCLConfigFileRequest is a vault HCL config create request
type CreateHCLConfigFileRequest struct {
	HCLConfig HCLable
	FilePath  string
	Chmod     string
	Chown     string
}

// CreateHCLConfigFileOpt is a functional option for a config create request
type CreateHCLConfigFileOpt func(*CreateHCLConfigFileRequest) *CreateHCLConfigFileRequest

// NewCreateHCLConfigFileRequest takes functional options and returns a new
// systemd unit request
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

// WithHCLConfigFile sets the config file to use
func WithHCLConfigFile(unit HCLable) CreateHCLConfigFileOpt {
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
