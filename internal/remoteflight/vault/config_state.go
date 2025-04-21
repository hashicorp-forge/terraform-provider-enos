// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	it "github.com/hashicorp-forge/terraform-provider-enos/internal/transport"
	"github.com/hashicorp-forge/terraform-provider-enos/internal/transport/command"
)

// ConfigStateSanitizedResponse is the sanitized config returned from vault.
type ConfigStateSanitizedResponse struct {
	Data *ConfigStateSanitizedResponseData `json:"data,omitempty"`
}

// ConfigStateSanitizedResponse is the data section of the sanitized config.
type ConfigStateSanitizedResponseData struct {
	APIAddr                   string            `json:"api_addr,omitempty"`
	CacheSize                 json.Number       `json:"cache_size,omitempty"`
	ClusterAddr               string            `json:"cluster_addr,omitempty"`
	ClusterCipherSuites       string            `json:"cluster_cipher_suites,omitempty"`
	ClusterName               string            `json:"cluster_name,omitempty"`
	DefaultLeaseTTL           json.Number       `json:"default_lease_ttl,omitempty"`
	DefaultMaxRequestDuration json.Number       `json:"default_max_request_duration,omitempty"`
	DisableCache              bool              `json:"disable_cache,omitempty"`
	DisableClustering         bool              `json:"disable_clustering,omitempty"`
	DisableIndexing           bool              `json:"disable_indexing,omitempty"`
	DisableMlock              bool              `json:"disable_mlock,omitempty"`
	DisablePerformanceStandby bool              `json:"disable_performance_standby,omitempty"`
	DisablePrintableCheck     bool              `json:"disable_printable_check,omitempty"`
	DisableSealwrap           bool              `json:"disable_sealwrap,omitempty"`
	EnableUI                  bool              `json:"enable_ui,omitempty"`
	Listeners                 []*ConfigListener `json:"listeners,omitempty"`
	LogFormat                 string            `json:"log_format,omitempty"`
	LogLevel                  string            `json:"log_level,omitempty"`
	MaxLeaseTTL               json.Number       `json:"max_lease_ttl,omitempty"`
	PIDFile                   string            `json:"pid_file,omitempty"`
	PluginDirectory           string            `json:"plugin_directory,omitempty"`
	RawStorageEndpoint        bool              `json:"raw_storage_endpoint,omitempty"`
	Seals                     []*ConfigSeals    `json:"seals,omitempty"`
	Storage                   *ConfigStorage    `json:"storage,omitempty"`
}

// ConfigListener is the listeners stanza of the configuration.
type ConfigListener struct {
	Config *ConfigListenerConfig `json:"config,omitempty"`
	Type   string                `json:"type,omitempty"`
}

// ConfigListenerConfig is the config section of the listeners configuration.
type ConfigListenerConfig struct {
	Address    string `json:"address,omitempty"`
	TLSDisable string `json:"tls_disable,omitempty"`
}

// ConfigSeals is the seals stanza of the configuration.
type ConfigSeals struct {
	Disabled bool   `json:"disabled,omitempty"`
	Type     string `json:"type,omitempty"`
}

// ConfigStorage is the storage stanza of the configuration.
type ConfigStorage struct {
	ClusterAddr       string `json:"cluster_addr,omitempty"`
	DisableClustering bool   `json:"disable_clustering,omitempty"`
	RedirectAddr      string `json:"redirect_addr,omitempty"`
	Type              string `json:"type,omitempty"`
}

// NewConfigStateSanitizedResponse returns a new instance of ConfigStateSanitizedResponse.
func NewConfigStateSanitizedResponse() *ConfigStateSanitizedResponse {
	return &ConfigStateSanitizedResponse{
		Data: &ConfigStateSanitizedResponseData{
			Listeners: []*ConfigListener{},
			Seals:     []*ConfigSeals{},
			Storage:   &ConfigStorage{},
		},
	}
}

// GetConfigStateSanitized returns a sanitized version of the configuration state.
func GetConfigStateSanitized(ctx context.Context, tr it.Transport, req *CLIRequest) (*ConfigStateSanitizedResponse, error) {
	var err error
	res := NewConfigStateSanitizedResponse()

	select {
	case <-ctx.Done():
		err = ctx.Err()
	default:
	}

	if req.BinPath == "" {
		err = errors.Join(err, errors.New("you must supply a vault bin path"))
	}

	if req.VaultAddr == "" {
		err = errors.Join(err, errors.New("you must supply a vault listen address"))
	}

	if req.Token == "" {
		err = errors.Join(err, errors.New("you must supply a vault token for the /v1/sys/config/state/sanitized endpoint"))
	}

	if err == nil {
		stdout, stderr, err1 := tr.Run(ctx, command.New(
			req.BinPath+" read -format=json sys/config/state/sanitized",
			command.WithEnvVar("VAULT_ADDR", req.VaultAddr),
			command.WithEnvVar("VAULT_TOKEN", req.Token),
		))
		if err1 != nil {
			err = err1
		}
		if stderr != "" {
			err = errors.Join(err, fmt.Errorf("unexpected write to stderr: %s", stderr))
		}

		// Deserialize the body onto our response.
		if stdout == "" {
			err = errors.Join(err, errors.New("no body was written to stdout"))
		} else {
			err = errors.Join(err, json.Unmarshal([]byte(stdout), res))
		}
	}

	if err != nil {
		return nil, errors.Join(errors.New("get config state sanitized: vault read sys/config/state/sanitized"))
	}

	return res, nil
}

// String returns the sanitized config.
func (s *ConfigStateSanitizedResponse) String() string {
	if s == nil || s.Data == nil {
		return ""
	}

	return s.Data.String()
}

// String returns the sanitized config data.
func (s *ConfigStateSanitizedResponseData) String() string {
	if s == nil {
		return ""
	}

	out := new(strings.Builder)

	_, _ = fmt.Fprintf(out, "API Addr: %s\n", s.APIAddr)
	_, _ = fmt.Fprintf(out, "Cache Size: %s\n", s.CacheSize)
	_, _ = fmt.Fprintf(out, "Cluster Addr: %s\n", s.ClusterAddr)
	_, _ = fmt.Fprintf(out, "Cluster Cipher Suites: %s\n", s.ClusterCipherSuites)
	_, _ = fmt.Fprintf(out, "Cluster Name: %s\n", s.ClusterName)
	_, _ = fmt.Fprintf(out, "Default Lease TTL: %s\n", s.DefaultLeaseTTL)
	_, _ = fmt.Fprintf(out, "Default Max Request Duration: %s\n", s.DefaultMaxRequestDuration)
	_, _ = fmt.Fprintf(out, "Disable Cache: %t\n", s.DisableCache)
	_, _ = fmt.Fprintf(out, "Disable Clustering: %t\n", s.DisableClustering)
	_, _ = fmt.Fprintf(out, "Disable Indexing: %t\n", s.DisableIndexing)
	_, _ = fmt.Fprintf(out, "Disable Mlock: %t\n", s.DisableMlock)
	_, _ = fmt.Fprintf(out, "Disable Performance Standby: %t\n", s.DisablePerformanceStandby)
	_, _ = fmt.Fprintf(out, "Disable Printable Check: %t\n", s.DisablePrintableCheck)
	_, _ = fmt.Fprintf(out, "Disable Sealwrap: %t\n", s.DisableSealwrap)
	_, _ = fmt.Fprintf(out, "Enable UI: %t\n", s.EnableUI)
	for i := range s.Listeners {
		if i == 0 {
			_, _ = fmt.Fprintln(out, "Listeners")
		}

		if s.Listeners[i] == nil {
			continue
		}

		_, _ = fmt.Fprintf(out, "  Type: %s\n", s.Listeners[i].Type)
		if s.Listeners[i].Config != nil {
			_, _ = fmt.Fprintf(out, "  Address: %s\n", s.Listeners[i].Config.Address)
			_, _ = fmt.Fprintf(out, "  TLS Disable: %s\n", s.Listeners[i].Config.TLSDisable)
		}
	}
	_, _ = fmt.Fprintf(out, "Log Format: %s\n", s.LogFormat)
	_, _ = fmt.Fprintf(out, "Log Level: %s\n", s.LogLevel)
	_, _ = fmt.Fprintf(out, "Max Lease TTL: %s\n", s.MaxLeaseTTL)
	_, _ = fmt.Fprintf(out, "PID File: %s\n", s.PIDFile)
	_, _ = fmt.Fprintf(out, "Plugin Directory: %s\n", s.PluginDirectory)
	_, _ = fmt.Fprintf(out, "Raw Storage Endpoint: %t\n", s.RawStorageEndpoint)
	for i := range s.Seals {
		if i == 0 {
			_, _ = fmt.Fprintln(out, "Seals")
		}

		if s.Seals[i] == nil {
			continue
		}

		if s.Seals[i] != nil {
			_, _ = fmt.Fprintf(out, " Disabled: %t\n", s.Seals[i].Disabled)
			_, _ = fmt.Fprintf(out, "  Type: %s\n", s.Seals[i].Type)
		}
	}
	_, _ = fmt.Fprintln(out, "Storage")
	_, _ = fmt.Fprintf(out, "  Cluster Addr: %s\n", s.Storage.ClusterAddr)
	_, _ = fmt.Fprintf(out, "  Disable Clustering: %t\n", s.Storage.DisableClustering)
	_, _ = fmt.Fprintf(out, "  Redirect Addr: %s\n", s.Storage.RedirectAddr)
	_, _ = fmt.Fprintf(out, "  Type: %s\n", s.Storage.Type)

	return out.String()
}
