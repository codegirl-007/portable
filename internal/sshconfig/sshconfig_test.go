package sshconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func useTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func configPath(home string) string {
	return filepath.Join(home, ".ssh", "config")
}

func TestEnsureCreatesConfig(t *testing.T) {
	home := useTempHome(t)
	if err := Ensure("my-sprite"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(configPath(home))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.Contains(got, "Host portable-my-sprite") {
		t.Fatalf("missing host block:\n%s", got)
	}
	if !strings.Contains(got, "ProxyCommand sprite proxy -s my-sprite -W 22") {
		t.Fatalf("missing proxy command:\n%s", got)
	}
}

func TestEnsureKeepsExistingHosts(t *testing.T) {
	home := useTempHome(t)
	p := configPath(home)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	original := "Host github.com\n  User git\n"
	if err := os.WriteFile(p, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Ensure("my-sprite"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.Contains(got, "Host github.com") {
		t.Fatalf("lost existing host:\n%s", got)
	}
	if !strings.Contains(got, "Host portable-my-sprite") {
		t.Fatalf("missing portable host:\n%s", got)
	}
}

func TestEnsureDoesNotOverwriteUnreadableConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file permissions")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can read mode 0 files")
	}
	home := useTempHome(t)
	p := configPath(home)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	original := "Host keep-me\n  User me\n"
	if err := os.WriteFile(p, []byte(original), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o600) })

	err := Ensure("my-sprite")
	if err == nil {
		t.Fatal("expected error when config is unreadable")
	}

	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != original {
		t.Fatalf("unreadable config was changed:\n%s", b)
	}
}
