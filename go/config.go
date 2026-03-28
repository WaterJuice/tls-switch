// ---------------------------------------------------------------------------------------
//
//	config.go
//	---------
//
//	Configuration types and atomic swap for hot reload. The config is received
//	from the Python wrapper and stored as an atomic pointer so new connections
//	pick up changes while existing connections continue unaffected.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Mar 2026 - Created
//
// ---------------------------------------------------------------------------------------
package main

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	"crypto/tls"
	"sync/atomic"
)

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
	Mode      string      // ModeTerminate or ModePassthrough
	Backend   string      // host:port
	TLSConfig *tls.Config // pre-built TLS config (terminate mode only)
}

// Config holds the complete server configuration.
type Config struct {
	ListenAddr string
	Hosts      map[string]*HostRoute
}

// ---------------------------------------------------------------------------------------
//
//	ConfigStore
//
// ---------------------------------------------------------------------------------------

// ConfigStore provides thread-safe access to the current configuration.
// Uses atomic.Pointer for lock-free reads on the per-connection hot path.
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
// Lookup returns the route for a hostname, or nil if not found.
func (cs *ConfigStore) Lookup(hostname string) *HostRoute {
	cfg := cs.config.Load()
	if cfg == nil {
		return nil
	}
	return cfg.Hosts[hostname]
}
