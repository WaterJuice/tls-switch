// ---------------------------------------------------------------------------------------
//
//	cli.go
//	------
//
//	CLI argument parsing, logging, config file watching, and signal handling.
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
	"encoding/json"
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
      "backend": "10.0.0.5:443",
      "proxy_protocol": "v2"
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
//	Run
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// Run is the main entry point called from main.go.
func Run(version string) {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "--help", "-h":
		printUsage()
	case "--version":
		fmt.Printf("tls-switch %s\n", version)
	case "--license":
		fmt.Print(licenceText)
	case "--example-config":
		fmt.Print(exampleConfig)
	case "-c", "--config":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: --config/-c requires a file path")
			os.Exit(1)
		}
		runServer(version, os.Args[2])
	default:
		fmt.Fprintf(os.Stderr, "Unknown option: %s\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "Run with --help for usage")
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------------------
func printUsage() {
	tty := isStdoutTerminal()
	if tty {
		h := "\033[1;34m"
		p := "\033[1;35m"
		s := "\033[32m"
		l := "\033[36m"
		m := "\033[33m"
		S := "\033[1;32m"
		L := "\033[1;36m"
		M := "\033[1;33m"
		r := "\033[0m"

		fmt.Printf("%susage: %s%stls-switch%s [%s-h%s] [%s--version%s] [%s--license%s] [%s--config %s%sFILE%s]\n", h, r, p, r, s, r, l, r, l, r, l, r, m, r)
		fmt.Printf("                  [%s--example-config%s]\n", l, r)
		fmt.Println()
		fmt.Println("SNI-based TLS reverse proxy.")
		fmt.Println()
		fmt.Printf("%soptions:%s\n", h, r)
		fmt.Printf("  %s-h%s, %s--help%s         show this help message and exit\n", S, r, L, r)
		fmt.Printf("  %s--version%s          show version and exit\n", L, r)
		fmt.Printf("  %s--license%s          show license information and exit\n", L, r)
		fmt.Printf("  %s--config%s, %s-c%s %sFILE%s  path to the JSON config file\n", L, r, S, r, M, r)
		fmt.Printf("  %s--example-config%s   print an example config file and exit\n", L, r)
	} else {
		fmt.Println("usage: tls-switch [-h] [--version] [--license] [--config FILE]")
		fmt.Println("                  [--example-config]")
		fmt.Println()
		fmt.Println("SNI-based TLS reverse proxy.")
		fmt.Println()
		fmt.Println("options:")
		fmt.Println("  -h, --help         show this help message and exit")
		fmt.Println("  --version          show version and exit")
		fmt.Println("  --license          show license information and exit")
		fmt.Println("  --config, -c FILE  path to the JSON config file")
		fmt.Println("  --example-config   print an example config file and exit")
	}
}

// ---------------------------------------------------------------------------------------
//
//	Server Runner
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func runServer(version string, configPath string) {
	isTTY := isTerminal()

	logInfo(isTTY, "tls-switch %s (go %s)", version, strings.TrimPrefix(runtime.Version(), "go"))

	configStore := NewConfigStore()
	server := NewServer(configStore, func(name string, data any) {
		logEvent(isTTY, name, data)
	})

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

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	stopWatch := make(chan struct{})
	go watchConfig(configPath, configStore, isTTY, stopWatch)

	logInfo(isTTY, "Ready (Ctrl+C to stop)")

	<-sigCh
	logInfo(isTTY, "Shutting down (Ctrl+C again to force)...")

	go func() {
		<-sigCh
		os.Exit(1)
	}()

	close(stopWatch)
	server.Stop()
	logInfo(isTTY, "Stopped")
}

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
func isStdoutTerminal() bool {
	info, err := os.Stdout.Stat()
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
