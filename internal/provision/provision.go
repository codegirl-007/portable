// Package provision builds the scripts that set up a Sprite (sshd, opencode)
// and install a project's dependencies.
package provision

import (
	"fmt"
	"strings"
)

// SetupRequest describes one-time Sprite setup.
type SetupRequest struct {
	PublicKey string
	GoAPIKey  string
	Model     string
	RemoteDir string
	Tools     []string
}

// DepsRequest describes dependency installation.
type DepsRequest struct {
	RemoteDir string
	Ensure    []string // tool-install commands (e.g. install bun)
	Install   []string // dependency-install commands (e.g. bun install)
}

// SetupScript renders the one-time Sprite setup script.
func SetupScript(req SetupRequest) string {
	var b strings.Builder
	w := func(format string, args ...any) { b.WriteString(fmt.Sprintf(format, args...) + "\n") }

	w("#!/usr/bin/env bash")
	w("set -euo pipefail")
	w(`export PATH="$HOME/.opencode/bin:$HOME/.local/bin:$PATH"`)
	w(`log() { printf '\n==> %%s\n' "$*"; }`)
	w("")
	w("REMOTE_DIR=%s", shq(req.RemoteDir))
	w(`mkdir -p "$REMOTE_DIR"`)
	w("")
	w(`log "sshd"`)
	w(`if [ ! -x /usr/sbin/sshd ]; then sudo apt-get update -qq && sudo apt-get install -y -qq openssh-server; fi`)
	w(`sprite-env services create sshd --cmd /usr/sbin/sshd >/dev/null 2>&1 || true`)
	w(`mkdir -p "$HOME/.ssh" && chmod 700 "$HOME/.ssh"`)
	w(`touch "$HOME/.ssh/authorized_keys" && chmod 600 "$HOME/.ssh/authorized_keys"`)
	if req.PublicKey != "" {
		w(`grep -qF %s "$HOME/.ssh/authorized_keys" 2>/dev/null || printf '%%s\n' %s >> "$HOME/.ssh/authorized_keys"`,
			shq(req.PublicKey), shq(req.PublicKey))
	}
	w("")
	w(`log "opencode"`)
	w(`command -v opencode >/dev/null 2>&1 || curl -fsSL https://opencode.ai/install | bash`)
	w(`grep -q '.opencode/bin' "$HOME/.bashrc" 2>/dev/null || echo 'export PATH="$HOME/.opencode/bin:$PATH"' >> "$HOME/.bashrc"`)
	w(`grep -q 'config/jevlint/env' "$HOME/.bashrc" 2>/dev/null || echo '[ -f "$HOME/.config/jevlint/env" ] && . "$HOME/.config/jevlint/env"' >> "$HOME/.bashrc"`)
	w(`grep -q 'config/jevlint/env' "$HOME/.profile" 2>/dev/null || echo '[ -f "$HOME/.config/jevlint/env" ] && . "$HOME/.config/jevlint/env"' >> "$HOME/.profile"`)
	w(`mkdir -p "$HOME/.config/opencode"`)
	if req.GoAPIKey != "" {
		w(`umask 077`)
		w(`cat > "$HOME/.config/opencode/env" <<'ENV'`)
		w(`export PATH="$HOME/.opencode/bin:$HOME/.local/bin:$PATH"`)
		w("export OPENCODE_API_KEY=%s", shq(req.GoAPIKey))
		w(`ENV`)
		w(`chmod 600 "$HOME/.config/opencode/env"`)
	}
	if req.Model != "" {
		w(`cat > "$HOME/.config/opencode/opencode.jsonc" <<'CFG'`)
		w(`{`)
		w(`  "$schema": "https://opencode.ai/config.json",`)
		w("  \"model\": %q", req.Model)
		w(`}`)
		w(`CFG`)
	}
	w("")
	w(`log "nvim"`)
	w(`if ! command -v nvim >/dev/null 2>&1; then`)
	w(`  cd /tmp`)
	w(`  curl -fsSL -o nvim.tar.gz https://github.com/neovim/neovim/releases/latest/download/nvim-linux-x86_64.tar.gz || curl -fsSL -o nvim.tar.gz https://github.com/neovim/neovim/releases/latest/download/nvim-linux64.tar.gz`)
	w(`  rm -rf "$HOME/.local/nvim"; mkdir -p "$HOME/.local/bin"`)
	w(`  tar xzf nvim.tar.gz && mv nvim-linux-* "$HOME/.local/nvim"`)
	w(`  ln -sf "$HOME/.local/nvim/bin/nvim" "$HOME/.local/bin/nvim"`)
	w(`  rm -f nvim.tar.gz`)
	w(`fi`)
	w(`command -v rg >/dev/null 2>&1 || sudo apt-get install -y -qq ripgrep >/dev/null 2>&1 || true`)
	w("")
	w(`log "tree-sitter"`)
	w(`if ! command -v tree-sitter >/dev/null 2>&1; then`)
	w(`  sudo apt-get install -y -qq unzip >/dev/null 2>&1 || true`)
	w(`  mkdir -p "$HOME/.local/bin"`)
	w(`  ts_ver=$(curl -s https://api.github.com/repos/tree-sitter/tree-sitter/releases/latest | grep -o '"tag_name": "[^"]*"' | cut -d'"' -f4)`)
	w(`  curl -fsSL -o /tmp/ts.zip "https://github.com/tree-sitter/tree-sitter/releases/download/$ts_ver/tree-sitter-cli-linux-x64.zip"`)
	w(`  ( cd /tmp && unzip -o ts.zip >/dev/null && cp tree-sitter "$HOME/.local/bin/tree-sitter" )`)
	w(`  chmod +x "$HOME/.local/bin/tree-sitter"`)
	w(`fi`)
	w("")
	w(`log "gh"`)
	w(`if ! command -v gh >/dev/null 2>&1; then`)
	w(`  v=$(curl -s https://api.github.com/repos/cli/cli/releases/latest | grep -o '"tag_name": "v[^"]*"' | cut -d'"' -f4)`)
	w(`  if [ -n "$v" ] && curl -fsSL -o /tmp/gh.tgz "https://github.com/cli/cli/releases/download/$v/gh_${v#v}_linux_amd64.tar.gz"; then`)
	w(`    tar xzf /tmp/gh.tgz -C /tmp && mkdir -p "$HOME/.local/bin" && cp "/tmp/gh_${v#v}_linux_amd64/bin/gh" "$HOME/.local/bin/gh" && chmod +x "$HOME/.local/bin/gh"`)
	w(`  else`)
	w(`    sudo apt-get install -y -qq gh >/dev/null 2>&1 || true`)
	w(`  fi`)
	w(`fi`)
	if len(req.Tools) > 0 {
		w("")
		w(`log "tools"`)
		w(`command -v gcc >/dev/null 2>&1 || sudo apt-get install -y -qq build-essential >/dev/null 2>&1 || true`)
		w(`export PATH="$HOME/.local/bin:$HOME/go/bin:$PATH"`)
		for _, cmd := range req.Tools {
			w("%s || echo 'warning: tool command failed'", cmd)
		}
	}
	w("")
	w(`touch "$HOME/.portable-provisioned"`)
	w(`log "provisioned"`)
	return b.String()
}

// DepsScript renders the dependency-installation script.
func DepsScript(req DepsRequest) string {
	var b strings.Builder
	w := func(format string, args ...any) { b.WriteString(fmt.Sprintf(format, args...) + "\n") }

	w("#!/usr/bin/env bash")
	w("set -euo pipefail")
	w(`export PATH="$HOME/.bun/bin:$HOME/.deno/bin:$HOME/.opencode/bin:$HOME/.local/bin:$PATH"`)
	w(`log() { printf '\n==> %%s\n' "$*"; }`)
	w("REMOTE_DIR=%s", shq(req.RemoteDir))
	w(`cd "$REMOTE_DIR"`)
	w("")
	w(`if [ -f go.mod ]; then log "go: go.mod"; go mod download; fi`)
	w(`if [ -f Cargo.toml ]; then log "rust: Cargo.toml"; cargo fetch || true; fi`)
	w(`if [ -f pyproject.toml ] || [ -f requirements.txt ]; then log "python"; pip install -r requirements.txt 2>/dev/null || true; fi`)
	if len(req.Ensure) > 0 {
		w("")
		for _, c := range req.Ensure {
			w("%s", c)
		}
	}
	if len(req.Install) > 0 {
		w("")
		w(`log "javascript dependencies"`)
		for _, c := range req.Install {
			w("%s", c)
		}
	}
	w("")
	w(`log "dependencies ready"`)
	return b.String()
}

func shq(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
