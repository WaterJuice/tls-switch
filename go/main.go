// ---------------------------------------------------------------------------------------
//
//	main.go
//	-------
//
//	Entry point for the tls-switch Go binary. Runs a JSON Lines protocol loop
//	over stdin/stdout — reads requests, dispatches to handlers, writes responses.
//	Also emits event lines for connection logging.
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
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------------------
//
//	Types
//
// ---------------------------------------------------------------------------------------

// Request is a JSON Lines request from the Python wrapper.
type Request struct {
	Command string          `json:"command"`
	Args    json.RawMessage `json:"args,omitempty"`
}

// Response is a JSON Lines response to the Python wrapper.
type Response struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Event is an unsolicited JSON Lines message sent from Go to Python.
type Event struct {
	Event string `json:"event"`
	Data  any    `json:"data,omitempty"`
}

const (
	statusOK    = "ok"
	statusError = "error"
)

// ---------------------------------------------------------------------------------------
func okResponse(data any) Response {
	return Response{Status: statusOK, Data: data}
}

// ---------------------------------------------------------------------------------------
func errResponse(msg string) Response {
	return Response{Status: statusError, Error: msg}
}

// ConfigureArgs is the args payload for the "configure" command.
type ConfigureArgs struct {
	ListenAddr string                   `json:"listen"`
	Hosts      map[string]ConfigureHost `json:"hosts"`
}

// ConfigureHost is a single host entry in the configure args.
type ConfigureHost struct {
	Mode    string `json:"mode"`
	Backend string `json:"backend"`
	CertPEM string `json:"cert_pem,omitempty"`
	KeyPEM  string `json:"key_pem,omitempty"`
}

// ---------------------------------------------------------------------------------------
//
//	Errors
//
// ---------------------------------------------------------------------------------------

var ErrNotConfigured = errors.New("server is not configured")

// ---------------------------------------------------------------------------------------
//
//	Global State
//
// ---------------------------------------------------------------------------------------

var (
	configStore = NewConfigStore()
	server      *Server
	outputMu    sync.Mutex
	encoder     *json.Encoder
)

// ---------------------------------------------------------------------------------------
// emitEvent sends an event line to stdout (thread-safe).
func emitEvent(name string, data any) {
	outputMu.Lock()
	defer outputMu.Unlock()
	encoder.Encode(Event{Event: name, Data: data})
}

// ---------------------------------------------------------------------------------------
// sendResponse sends a response line to stdout (thread-safe).
func sendResponse(resp Response) error {
	outputMu.Lock()
	defer outputMu.Unlock()
	return encoder.Encode(resp)
}

// ---------------------------------------------------------------------------------------
//
//	Command Handlers
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func handleConfigure(args json.RawMessage) Response {
	var ca ConfigureArgs
	if err := json.Unmarshal(args, &ca); err != nil {
		return errResponse("invalid configure args: " + err.Error())
	}

	if ca.ListenAddr == "" {
		return errResponse("listen address is required")
	}

	hosts := make(map[string]*HostRoute, len(ca.Hosts))
	for hostname, h := range ca.Hosts {
		route := &HostRoute{
			Mode:    h.Mode,
			Backend: h.Backend,
		}

		if h.Mode == ModeTerminate {
			cert, err := tls.X509KeyPair([]byte(h.CertPEM), []byte(h.KeyPEM))
			if err != nil {
				return errResponse("failed to load cert/key for " + hostname + ": " + err.Error())
			}
			route.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			}
		}

		hosts[hostname] = route
	}

	configStore.Set(&Config{
		ListenAddr: ca.ListenAddr,
		Hosts:      hosts,
	})

	return okResponse(map[string]any{"hosts": len(hosts)})
}

// ---------------------------------------------------------------------------------------
func handleStart(args json.RawMessage) Response {
	if err := server.Start(); err != nil {
		return errResponse("failed to start: " + err.Error())
	}
	cfg := configStore.Get()
	addr := ""
	if cfg != nil {
		addr = cfg.ListenAddr
	}
	return okResponse(map[string]any{"listening": addr})
}

// ---------------------------------------------------------------------------------------
func handleStop(args json.RawMessage) Response {
	server.Stop()
	return okResponse(nil)
}

// ---------------------------------------------------------------------------------------
func handleStatus(args json.RawMessage) Response {
	cfg := configStore.Get()
	configured := cfg != nil
	running := server.IsRunning()
	activeConns := server.ActiveConnections()

	listenAddr := ""
	hostCount := 0
	if cfg != nil {
		listenAddr = cfg.ListenAddr
		hostCount = len(cfg.Hosts)
	}

	return okResponse(map[string]any{
		"configured":   configured,
		"running":      running,
		"listen":       listenAddr,
		"hosts":        hostCount,
		"active_conns": activeConns,
	})
}

// ---------------------------------------------------------------------------------------
func handleReload(args json.RawMessage) Response {
	return handleConfigure(args)
}

// ---------------------------------------------------------------------------------------
func handleVersion(args json.RawMessage) Response {
	return okResponse(map[string]string{
		"go":   strings.TrimPrefix(runtime.Version(), "go"),
		"os":   runtime.GOOS,
		"arch": runtime.GOARCH,
	})
}

// ---------------------------------------------------------------------------------------
//
//	Command Dispatch
//
// ---------------------------------------------------------------------------------------

var commands = map[string]func(json.RawMessage) Response{
	"configure": handleConfigure,
	"start":     handleStart,
	"stop":      handleStop,
	"status":    handleStatus,
	"reload":    handleReload,
	"version":   handleVersion,
}

// ---------------------------------------------------------------------------------------
//
//	Main
//
// ---------------------------------------------------------------------------------------

const maxLineSize = 1024 * 1024 // 1MB max request size

// ---------------------------------------------------------------------------------------
func main() {
	encoder = json.NewEncoder(os.Stdout)
	server = NewServer(configStore, emitEvent)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, maxLineSize), maxLineSize)

	for scanner.Scan() {
		line := scanner.Bytes()

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			if err := sendResponse(errResponse("invalid request: expected JSON object")); err != nil {
				return
			}
			continue
		}

		handler, ok := commands[req.Command]
		if !ok {
			if err := sendResponse(errResponse("unknown command: " + req.Command)); err != nil {
				return
			}
			continue
		}

		resp := handler(req.Args)
		if err := sendResponse(resp); err != nil {
			return
		}
	}
}

// ---------------------------------------------------------------------------------------
// formatTimestamp returns an ISO 8601 timestamp string.
func formatTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
