package provision

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupScript(t *testing.T) {
	s := SetupScript(SetupRequest{
		PublicKey: "ssh-ed25519 AAAA test@host",
		GoAPIKey:  "abc123",
		Model:     "opencode-go/deepseek-v4.1-flash",
		RemoteDir: "/home/sprite/app",
	})
	for _, want := range []string{
		"set -euo pipefail",
		"openssh-server",
		"sprite-env services create sshd",
		"ssh-ed25519 AAAA test@host",
		"opencode.ai/install",
		"nvim-linux-x86_64.tar.gz",
		`ln -sf "$HOME/.local/nvim/bin/nvim" "$HOME/.local/bin/nvim"`,
		"OPENCODE_API_KEY='abc123'",
		`"model": "opencode-go/deepseek-v4.1-flash"`,
		"/home/sprite/app",
		`touch "$HOME/.portable-provisioned"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("setup script missing %q\n--- script ---\n%s", want, s)
		}
	}
}

func TestDepsScript(t *testing.T) {
	s := DepsScript(DepsRequest{
		RemoteDir: "/home/sprite/app",
		Ensure:    []string{"echo ensure-tool"},
		Install:   []string{"npm install"},
	})
	for _, want := range []string{
		`REMOTE_DIR='/home/sprite/app'`,
		`cd "$REMOTE_DIR"`,
		"go mod download",
		"echo ensure-tool",
		"npm install",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("deps script missing %q\n--- script ---\n%s", want, s)
		}
	}
}

func TestShqEscapesQuotes(t *testing.T) {
	if got := shq("it's"); got != `'it'\''s'` {
		t.Fatalf("shq = %q", got)
	}
}

func TestScriptSyntax(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	scripts := map[string]string{
		"setup": SetupScript(SetupRequest{
			PublicKey: "ssh-ed25519 AAAA test@host",
			GoAPIKey:  "abc123",
			Model:     "opencode-go/deepseek-v4.1-flash",
			RemoteDir: "/home/sprite/app",
			Tools:     []string{"echo tool"},
		}),
		"deps": DepsScript(DepsRequest{
			RemoteDir: "/home/sprite/app",
			Ensure:    []string{"echo ensure"},
			Install:   []string{"npm install"},
		}),
	}
	for name, script := range scripts {
		f := filepath.Join(t.TempDir(), name+".sh")
		if err := os.WriteFile(f, []byte(script), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(bash, "-n", f).CombinedOutput(); err != nil {
			t.Errorf("%s script syntax error: %v\n%s", name, err, out)
		}
	}
}
