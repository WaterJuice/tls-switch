// ---------------------------------------------------------------------------------------
//
//	config.go
//	---------
//
//	Configuration types, loading, validation, and atomic swap for hot reload.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Mar 2026 - Created
//
// ---------------------------------------------------------------------------------------
package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

var ErrNotConfigured = errors.New("server is not configured")

// ---------------------------------------------------------------------------------------
//
//	Constants
//
// ---------------------------------------------------------------------------------------

const (
	ModeTerminate   = "terminate"
	ModePassthrough = "passthrough"
)

// ---------------------------------------------------------------------------------------
//
//	Types
//
// ---------------------------------------------------------------------------------------

// HostRoute defines the routing configuration for a single hostname.
type HostRoute struct {
	Mode      string
	Backend   string
	TLSConfig *tls.Config
}

// Config holds the complete server configuration.
type Config struct {
	ListenAddr string
	Hosts      map[string]*HostRoute
}

// ConfigStore provides thread-safe access to the current configuration.
type ConfigStore struct {
	config atomic.Pointer[Config]
}

// ---------------------------------------------------------------------------------------
func NewConfigStore() *ConfigStore {
	return &ConfigStore{}
}

// ---------------------------------------------------------------------------------------
func (cs *ConfigStore) Get() *Config {
	return cs.config.Load()
}

// ---------------------------------------------------------------------------------------
func (cs *ConfigStore) Set(cfg *Config) {
	cs.config.Store(cfg)
}

// ---------------------------------------------------------------------------------------
func (cs *ConfigStore) Lookup(hostname string) *HostRoute {
	cfg := cs.config.Load()
	if cfg == nil {
		return nil
	}
	return cfg.Hosts[hostname]
}

// ---------------------------------------------------------------------------------------
func (cs *ConfigStore) AnyTLSConfig() *tls.Config {
	cfg := cs.config.Load()
	if cfg == nil {
		return nil
	}
	for _, route := range cfg.Hosts {
		if route.TLSConfig != nil {
			return route.TLSConfig
		}
	}
	return nil
}

// ---------------------------------------------------------------------------------------
//
//	Config Loading
//
// ---------------------------------------------------------------------------------------

// configFile is the JSON structure of the config file.
type configFile struct {
	Listen string                `json:"listen"`
	Hosts  map[string]configHost `json:"hosts"`
}

type configHost struct {
	Mode    string `json:"mode"`
	Backend string `json:"backend"`
	Cert    string `json:"cert,omitempty"`
	Key     string `json:"key,omitempty"`
}

// ---------------------------------------------------------------------------------------
// LoadConfig reads and validates a config file, returning a ready-to-use Config.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cf configFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("invalid JSON in config file: %w", err)
	}

	if cf.Listen == "" {
		return nil, fmt.Errorf("'listen' is required")
	}

	if len(cf.Hosts) == 0 {
		return nil, fmt.Errorf("'hosts' must contain at least one entry")
	}

	baseDir := filepath.Dir(path)
	hosts := make(map[string]*HostRoute, len(cf.Hosts))

	for hostname, h := range cf.Hosts {
		if h.Mode != ModeTerminate && h.Mode != ModePassthrough {
			return nil, fmt.Errorf("host '%s': mode must be 'terminate' or 'passthrough'", hostname)
		}

		if h.Backend == "" {
			return nil, fmt.Errorf("host '%s': backend is required", hostname)
		}

		parts := strings.SplitN(h.Backend, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("host '%s': backend must be in host:port format", hostname)
		}

		route := &HostRoute{
			Mode:    h.Mode,
			Backend: h.Backend,
		}

		if h.Mode == ModeTerminate {
			if h.Cert == "" {
				return nil, fmt.Errorf("host '%s': cert is required for terminate mode", hostname)
			}
			if h.Key == "" {
				return nil, fmt.Errorf("host '%s': key is required for terminate mode", hostname)
			}

			certPath := resolvePath(h.Cert, baseDir)
			keyPath := resolvePath(h.Key, baseDir)

			certPEM, err := os.ReadFile(certPath)
			if err != nil {
				return nil, fmt.Errorf("host '%s': failed to read cert file: %w", hostname, err)
			}
			keyPEM, err := os.ReadFile(keyPath)
			if err != nil {
				return nil, fmt.Errorf("host '%s': failed to read key file: %w", hostname, err)
			}

			cert, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				return nil, fmt.Errorf("host '%s': cert/key validation failed: %w", hostname, err)
			}

			route.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			}
		}

		hosts[hostname] = route
	}

	return &Config{
		ListenAddr: cf.Listen,
		Hosts:      hosts,
	}, nil
}

// ---------------------------------------------------------------------------------------
func resolvePath(p string, baseDir string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(baseDir, p)
}
