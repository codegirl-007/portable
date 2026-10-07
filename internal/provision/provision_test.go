package provision

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codegirl-007/portable/internal/config"
)

func TestSetupScriptMinimal(t *testing.T) {
	s := SetupScript(SetupRequest{
		PublicKey: "ssh-ed25519 AAAA test@host",
		RemoteDir: "/home/sprite/app",
		Ripgrep:   true,
	})
	for _, want := range []string{
		"set -euo pipefail",
		"openssh-server",
		"sprite-env services create sshd",
		"ssh-ed25519 AAAA test@host",
		"/home/sprite/app",
		`touch "$HOME/.portable-provisioned"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("setup script missing %q\n--- script ---\n%s", want, s)
		}
	}
	for _, absent := range []string{
		"opencode.ai/install",
		"nvim-linux-x86_64.tar.gz",
	} {
		if strings.Contains(s, absent) {
			t.Errorf("minimal setup should not contain %q", absent)
		}
	}
}

func TestSetupScriptCredentialMkdir(t *testing.T) {
	s := SetupScript(SetupRequest{
		RemoteDir: "/home/sprite/app",
		CredentialFiles: []CredentialFile{
			{RelPath: ".config/portable/agent.env", Content: "export CURSOR_API_KEY='x'\n"},
		},
	})
	if strings.Contains(s, `mkdir -p "$HOME/'.config/portable'"`) {
		t.Fatalf("mkdir must not shell-quote the directory segment\n%s", s)
	}
	if !strings.Contains(s, `mkdir -p "$HOME/.config/portable"`) {
		t.Fatalf("missing mkdir for credential dir\n%s", s)
	}
}

func TestSetupScriptOpenCode(t *testing.T) {
	s := SetupScript(SetupRequest{
		PublicKey: "ssh-ed25519 AAAA test@host",
		RemoteDir: "/home/sprite/app",
		AgentInstall: []string{
			`command -v opencode >/dev/null 2>&1 || curl -fsSL https://opencode.ai/install | bash`,
		},
		CredentialFiles: []CredentialFile{
			{RelPath: ".config/opencode/env", Content: "export OPENCODE_API_KEY='abc123'\n"},
		},
		OpenCodeModel: "opencode-go/deepseek-v4.1-flash",
		Nvim:          true,
	})
	for _, want := range []string{
		"opencode.ai/install",
		"nvim-linux-x86_64.tar.gz",
		"OPENCODE_API_KEY='abc123'",
		`"model": "opencode-go/deepseek-v4.1-flash"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("setup script missing %q\n--- script ---\n%s", want, s)
		}
	}
}

func TestNewSetupRequestFromConfig(t *testing.T) {
	cfg := &config.Config{
		DefaultAgent: "opencode",
		Agents: []config.Agent{{
			Name: "opencode",
			Install: []string{
				"install-opencode",
			},
			Credential: "opencode",
		}},
		Provision: config.Provision{Nvim: true},
		Dotfiles:  []string{".config/nvim"},
		Tools:     []string{"echo tool"},
	}
	req := NewSetupRequest(cfg, SetupRequestFromConfig{
		PublicKey: "pk",
		RemoteDir: "/home/sprite/app",
	})
	if !strings.Contains(strings.Join(req.AgentInstall, " "), "install-opencode") {
		t.Fatalf("AgentInstall = %v", req.AgentInstall)
	}
	if !req.Nvim || !req.NvimBootstrapDot {
		t.Fatalf("nvim bootstrap: nvim=%v bootstrap=%v", req.Nvim, req.NvimBootstrapDot)
	}
}

func TestDepsScript(t *testing.T) {
	s := DepsScript(DepsRequest{
		RemoteDir:       "/home/sprite/app",
		Ensure:          []string{"echo ensure-tool"},
		Install:         []string{"npm install"},
		AutoLangInstall: true,
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
			RemoteDir: "/home/sprite/app",
			AgentInstall: []string{
				`command -v opencode >/dev/null 2>&1 || true`,
			},
			CredentialFiles: []CredentialFile{
				{RelPath: ".config/opencode/env", Content: "export OPENCODE_API_KEY='x'\n"},
			},
			Nvim:    true,
			Ripgrep: true,
			Tools:   []string{"echo tool"},
		}),
		"deps": DepsScript(DepsRequest{
			RemoteDir:       "/home/sprite/app",
			Ensure:          []string{"echo ensure"},
			Install:         []string{"npm install"},
			AutoLangInstall: true,
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
