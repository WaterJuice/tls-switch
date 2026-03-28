// ---------------------------------------------------------------------------------------
//
//	main.go
//	-------
//
//	Entry point for the tls-switch binary. Parses CLI flags, loads config,
//	starts the server, watches for config changes, and handles shutdown.
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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// ---------------------------------------------------------------------------------------
//
//	Constants
//
// ---------------------------------------------------------------------------------------

// Version is set at build time via -ldflags
var Version = "dev"

const exampleConfig = `{
  "listen": ":443",
  "hosts": {
    "app.example.com": {
      "mode": "terminate",
      "cert": "/etc/tls-switch/app.example.com.crt",
      "key": "/etc/tls-switch/app.example.com.key",
      "backend": "127.0.0.1:8080"
    },
    "legacy.example.com": {
      "mode": "passthrough",
      "backend": "10.0.0.5:443"
    }
  }
}
`

const licenceText = `tls-switch — Released under the Unlicense (public domain)

This is free and unencumbered software released into the public domain.

Anyone is free to copy, modify, publish, use, compile, sell, or
distribute this software, either in source code form or as a compiled
binary, for any purpose, commercial or non-commercial, and by any
means.

For more information, please refer to <https://unlicense.org/>
`

const configPollInterval = 2 * time.Second

// ---------------------------------------------------------------------------------------
//
//	Errors
//
// ---------------------------------------------------------------------------------------

var ErrNotConfigured = errors.New("server is not configured")

// ---------------------------------------------------------------------------------------
//
//	Main
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "--help", "-h":
		printUsage()
	case "--version":
		fmt.Printf("tls-switch: %s\n", Version)
		fmt.Printf("go: %s\n", strings.TrimPrefix(runtime.Version(), "go"))
	case "--license":
		fmt.Print(licenceText)
	case "--example-config":
		fmt.Print(exampleConfig)
	case "-c", "--config":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: --config/-c requires a file path")
			os.Exit(1)
		}
		runServer(os.Args[2])
	default:
		fmt.Fprintf(os.Stderr, "Unknown option: %s\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "Run with --help for usage")
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------------------
func printUsage() {
	fmt.Println("usage: tls-switch [--help] [--version] [--license] [--config FILE]")
	fmt.Println("                  [--example-config]")
	fmt.Println()
	fmt.Println("SNI-based TLS reverse proxy.")
	fmt.Println()
	fmt.Println("options:")
	fmt.Println("  -h, --help        show this help message and exit")
	fmt.Println("  --version         show version and exit")
	fmt.Println("  --license         show license information and exit")
	fmt.Println("  --config, -c FILE path to the JSON config file")
	fmt.Println("  --example-config  print an example config file and exit")
}

// ---------------------------------------------------------------------------------------
func runServer(configPath string) {
	isTTY := isTerminal()

	logInfo(isTTY, "tls-switch %s (go %s)", Version, strings.TrimPrefix(runtime.Version(), "go"))

	configStore := NewConfigStore()
	server := NewServer(configStore, func(name string, data any) {
		logEvent(isTTY, name, data)
	})

	// Load initial config
	cfg, err := loadAndApplyConfig(configPath, configStore, isTTY)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %s\n", err)
		os.Exit(1)
	}

	logInfo(isTTY, "Listening on %s", cfg.ListenAddr)

	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start: %s\n", err)
		os.Exit(1)
	}

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start config file watcher
	stopWatch := make(chan struct{})
	go watchConfig(configPath, configStore, isTTY, stopWatch)

	logInfo(isTTY, "Ready (Ctrl+C to stop)")

	// Wait for first signal
	<-sigCh
	logInfo(isTTY, "Shutting down (Ctrl+C again to force)...")

	// Second signal force-kills
	go func() {
		<-sigCh
		os.Exit(1)
	}()

	close(stopWatch)
	server.Stop()
	logInfo(isTTY, "Stopped")
}

// ---------------------------------------------------------------------------------------
//
//	Config Loading
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func loadAndApplyConfig(path string, cs *ConfigStore, isTTY bool) (*Config, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	cs.Set(cfg)

	logInfo(isTTY, "Configured %d host(s):", len(cfg.Hosts))
	for hostname, route := range cfg.Hosts {
		logInfo(isTTY, "  %s (%s) → %s",
			colorCyan(hostname, isTTY),
			route.Mode,
			colorGreen(route.Backend, isTTY))
	}

	return cfg, nil
}

// ---------------------------------------------------------------------------------------
//
//	Config File Watching
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func watchConfig(path string, cs *ConfigStore, isTTY bool, stop chan struct{}) {
	mtimes := getFileMtimes(path)

	for {
		select {
		case <-stop:
			return
		case <-time.After(configPollInterval):
		}

		newMtimes := getFileMtimes(path)
		if mtimesEqual(mtimes, newMtimes) {
			continue
		}
		mtimes = newMtimes

		logInfo(isTTY, "Config change detected, reloading...")

		_, err := loadAndApplyConfig(path, cs, isTTY)
		if err != nil {
			logInfo(isTTY, "Reload failed (keeping current config): %s", err)
		}
	}
}

// ---------------------------------------------------------------------------------------
func getFileMtimes(configPath string) map[string]time.Time {
	mtimes := make(map[string]time.Time)

	info, err := os.Stat(configPath)
	if err != nil {
		return mtimes
	}
	mtimes[configPath] = info.ModTime()

	data, err := os.ReadFile(configPath)
	if err != nil {
		return mtimes
	}

	var raw struct {
		Hosts map[string]struct {
			Cert string `json:"cert"`
			Key  string `json:"key"`
		} `json:"hosts"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return mtimes
	}

	for _, h := range raw.Hosts {
		for _, f := range []string{h.Cert, h.Key} {
			if f == "" {
				continue
			}
			if info, err := os.Stat(f); err == nil {
				mtimes[f] = info.ModTime()
			}
		}
	}

	return mtimes
}

// ---------------------------------------------------------------------------------------
func mtimesEqual(a, b map[string]time.Time) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || !v.Equal(bv) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------------------
//
//	Logging
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func logInfo(isTTY bool, format string, args ...any) {
	ts := time.Now().Format("2006-01-02 15:04:05 -0700")
	msg := fmt.Sprintf(format, args...)
	if isTTY {
		fmt.Fprintf(os.Stderr, "\033[2m%s\033[0m %s\n", ts, msg)
	} else {
		fmt.Fprintf(os.Stderr, "%s %s\n", ts, msg)
	}
}

// ---------------------------------------------------------------------------------------
func logEvent(isTTY bool, event string, data any) {
	if event != "connection" {
		return
	}
	m, ok := data.(map[string]string)
	if !ok {
		return
	}

	source := m["source"]
	hostname := m["hostname"]
	mode := m["mode"]
	backend := m["backend"]
	action := m["action"]
	reason := m["reason"]
	errMsg := m["error"]

	if errMsg != "" {
		logInfo(isTTY, "%s %s", colorDim(source, isTTY), colorRed(errMsg, isTTY))
	} else if action == "rejected" {
		logInfo(isTTY, "%s → %s %s",
			colorDim(source, isTTY),
			colorYellow(hostname, isTTY),
			colorRed(reason, isTTY))
	} else {
		logInfo(isTTY, "%s → %s %s → %s",
			colorDim(source, isTTY),
			colorCyan(hostname, isTTY),
			colorDim("("+mode+")", isTTY),
			colorGreen(backend, isTTY))
	}
}

// ---------------------------------------------------------------------------------------
func isTerminal() bool {
	info, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// ---------------------------------------------------------------------------------------
func colorDim(s string, tty bool) string {
	if !tty {
		return s
	}
	return "\033[2m" + s + "\033[0m"
}

// ---------------------------------------------------------------------------------------
func colorCyan(s string, tty bool) string {
	if !tty {
		return s
	}
	return "\033[36m" + s + "\033[0m"
}

// ---------------------------------------------------------------------------------------
func colorGreen(s string, tty bool) string {
	if !tty {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

// ---------------------------------------------------------------------------------------
func colorYellow(s string, tty bool) string {
	if !tty {
		return s
	}
	return "\033[33m" + s + "\033[0m"
}

// ---------------------------------------------------------------------------------------
func colorRed(s string, tty bool) string {
	if !tty {
		return s
	}
	return "\033[31m" + s + "\033[0m"
}
