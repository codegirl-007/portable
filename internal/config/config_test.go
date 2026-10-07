package config

import "testing"

func TestSetupDone(t *testing.T) {
	if (&Config{SetupVersion: 1, DefaultAgent: "opencode"}).SetupDone() != true {
		t.Fatal("expected done")
	}
	if (&Config{SetupVersion: 0, DefaultAgent: "opencode"}).SetupDone() != false {
		t.Fatal("expected not done without version")
	}
	if (&Config{SetupVersion: 1, DefaultAgent: ""}).SetupDone() != false {
		t.Fatal("expected not done without agent")
	}
}

func TestRemoteShellPreambleSourcesEnv(t *testing.T) {
	c := &Config{
		EnvFiles: []string{".config/opencode/env"},
	}
	p := c.RemoteShellPreamble()
	if p == "" || p[len(p)-2:] != "; " {
		t.Fatalf("preamble = %q", p)
	}
	if !containsAll(p, ".config/opencode/env", "set -a", "export PATH=") {
		t.Fatalf("preamble = %q", p)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
