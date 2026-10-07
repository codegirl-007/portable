// Package project interprets per-project .portable.yaml (tools and install steps).
package project

import (
	"fmt"
	"strings"
)

// Config is the project-local .portable.yaml (not global setup).
type Config struct {
	Tools   []string `mapstructure:"tools"`
	Ensure  []string `mapstructure:"ensure"`
	Install []string `mapstructure:"install"`
}

// Defined reports whether the project declares its own tooling or install steps.
func (c Config) Defined() bool {
	return len(c.Tools) > 0 || len(c.Ensure) > 0 || len(c.Install) > 0
}

// EnsureCommands returns idempotent shell snippets to install named tools on the workspace.
func EnsureCommands(toolNames []string) ([]string, error) {
	var out []string
	for _, name := range toolNames {
		name = strings.ToLower(strings.TrimSpace(name))
		cmds, ok := ensureByTool[name]
		if !ok {
			return nil, fmt.Errorf("unknown tool %q in .portable.yaml (supported: %s)", name, SupportedTools())
		}
		out = append(out, cmds...)
	}
	return out, nil
}

// SupportedTools lists valid tool names for .portable.yaml.
func SupportedTools() string {
	return "bun, deno, node, npm, pnpm, yarn, go, rust"
}

var ensureByTool = map[string][]string{
	"bun": {
		`command -v bun >/dev/null 2>&1 || curl -fsSL https://bun.sh/install | bash`,
	},
	"deno": {
		`command -v deno >/dev/null 2>&1 || curl -fsSL https://deno.land/install.sh | sh`,
	},
	"node": {
		`command -v node >/dev/null 2>&1 || curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - && sudo apt-get install -y -qq nodejs`,
	},
	"npm": {
		`command -v npm >/dev/null 2>&1 || curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - && sudo apt-get install -y -qq nodejs`,
	},
	"pnpm": {
		`command -v node >/dev/null 2>&1 || curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - && sudo apt-get install -y -qq nodejs`,
		`command -v pnpm >/dev/null 2>&1 || corepack enable >/dev/null 2>&1 || npm i -g pnpm`,
	},
	"yarn": {
		`command -v node >/dev/null 2>&1 || curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - && sudo apt-get install -y -qq nodejs`,
		`command -v yarn >/dev/null 2>&1 || corepack enable >/dev/null 2>&1 || npm i -g yarn`,
	},
	"go": {
		`command -v go >/dev/null 2>&1 || sudo apt-get update -qq && sudo apt-get install -y -qq golang-go`,
	},
	"rust": {
		`command -v cargo >/dev/null 2>&1 || curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y`,
		`grep -q '.cargo/env' "$HOME/.bashrc" 2>/dev/null || echo '. "$HOME/.cargo/env"' >> "$HOME/.bashrc"`,
	},
}

// DepsPlan is the resolved ensure/install run after sync on portable up.
type DepsPlan struct {
	Ensure          []string
	Install         []string
	AutoLangInstall bool // go mod download, cargo fetch, etc.
}

// DepsPlanFromConfig builds the dependency phase from project config, with optional detect fallback.
func DepsPlanFromConfig(proj Config, fallback func() (ensure, install []string)) (DepsPlan, error) {
	if !proj.Defined() {
		e, i := fallback()
		return DepsPlan{Ensure: e, Install: i, AutoLangInstall: true}, nil
	}
	ensure, err := EnsureCommands(proj.Tools)
	if err != nil {
		return DepsPlan{}, err
	}
	ensure = append(ensure, proj.Ensure...)
	return DepsPlan{
		Ensure:          ensure,
		Install:         append([]string(nil), proj.Install...),
		AutoLangInstall: false,
	}, nil
}
