// ---------------------------------------------------------------------------------------
//
//	config_test.go
//	--------------
//
//	Tests for config loading and validation.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
// ---------------------------------------------------------------------------------------
package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------------------
//
//	proxy_protocol field
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func TestLoadConfig_RejectsUnknownProxyProtocol(t *testing.T) {
	path := writeTempConfig(t, `{
        "listen": ":443",
        "hosts": {
            "example.com": {
                "mode": "passthrough",
                "backend": "127.0.0.1:8080",
                "proxy_protocol": "v3"
            }
        }
    }`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := `host 'example.com': proxy_protocol must be 'v1', 'v2', or omitted (got "v3")`
	if err.Error() != want {
		t.Errorf("got  %q\nwant %q", err.Error(), want)
	}
}

// ---------------------------------------------------------------------------------------
func TestLoadConfig_AcceptsValidProxyProtocol(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"v1", "v1"},
		{"v2", "v2"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTempConfig(t, fmt.Sprintf(`{
                "listen": ":443",
                "hosts": {
                    "example.com": {
                        "mode": "passthrough",
                        "backend": "127.0.0.1:8080",
                        "proxy_protocol": "%s"
                    }
                }
            }`, tc.value))

			cfg, err := LoadConfig(path)
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if got := cfg.Hosts["example.com"].ProxyProtocol; got != tc.value {
				t.Errorf("ProxyProtocol = %q, want %q", got, tc.value)
			}
		})
	}
}

// ---------------------------------------------------------------------------------------
func TestLoadConfig_OmittedProxyProtocolDefaultsToOff(t *testing.T) {
	path := writeTempConfig(t, `{
        "listen": ":443",
        "hosts": {
            "example.com": {
                "mode": "passthrough",
                "backend": "127.0.0.1:8080"
            }
        }
    }`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got := cfg.Hosts["example.com"].ProxyProtocol; got != ProxyProtocolOff {
		t.Errorf("ProxyProtocol = %q, want %q", got, ProxyProtocolOff)
	}
}

// ---------------------------------------------------------------------------------------
//
//	Helpers
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}
