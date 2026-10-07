// Package opencode reads the local OpenCode configuration that portable needs
// to run opencode on a workspace droplet: the OpenCode Go API key and the
// default model.
package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func dataDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "opencode")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "opencode")
}

func stateDir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "opencode")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "opencode")
}

// DBPath is the local opencode database.
func DBPath() string { return filepath.Join(dataDir(), "opencode.db") }

// GoAPIKey returns the OpenCode Go API key, from the OPENCODE_API_KEY
// environment variable or the local opencode database.
func GoAPIKey() (string, error) {
	if k := strings.TrimSpace(os.Getenv("OPENCODE_API_KEY")); k != "" {
		return k, nil
	}
	if k := keyFromSQLite(); k != "" {
		return k, nil
	}
	if k := keyFromScan(); k != "" {
		return k, nil
	}
	return "", fmt.Errorf("no OpenCode Go key found; set OPENCODE_API_KEY or connect OpenCode Go in opencode")
}

func keyFromSQLite() string {
	bin, err := exec.LookPath("sqlite3")
	if err != nil {
		return ""
	}
	out, err := exec.Command(bin, "-readonly", DBPath(),
		"select value from credential where integration_id='opencode-go' and active=1 limit 1").Output()
	if err != nil {
		return ""
	}
	return keyFromValue(strings.TrimSpace(string(out)))
}

func keyFromValue(v string) string {
	var c struct {
		Key string `json:"key"`
	}
	if json.Unmarshal([]byte(v), &c) == nil {
		return c.Key
	}
	return ""
}

var keyRe = regexp.MustCompile(`\{"type":"[a-z]+","key":"([^"]{20,})"\}`)

// keyFromScan is a dependency-free fallback that scans the database (and its
// write-ahead log) for the stored credential value.
func keyFromScan() string {
	for _, f := range []string{DBPath(), DBPath() + "-wal"} {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if m := keyRe.FindSubmatch(b); m != nil {
			return string(m[1])
		}
	}
	return ""
}

// DefaultModel returns the most recent model as "provider/model", if known.
func DefaultModel() string {
	b, err := os.ReadFile(filepath.Join(stateDir(), "model.json"))
	if err != nil {
		return ""
	}
	var s struct {
		Recent []struct {
			ProviderID string `json:"providerID"`
			ModelID    string `json:"modelID"`
		} `json:"recent"`
	}
	if json.Unmarshal(b, &s) != nil || len(s.Recent) == 0 {
		return ""
	}
	r := s.Recent[0]
	if r.ProviderID == "" || r.ModelID == "" {
		return ""
	}
	return r.ProviderID + "/" + r.ModelID
}
