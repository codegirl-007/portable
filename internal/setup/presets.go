// Package setup implements the interactive portable setup wizard and agent presets.
package setup

import (
	"os"
	"strings"

	"github.com/codegirl-007/portable/internal/config"
)

// Choice is one selectable coding agent in the wizard.
type Choice struct {
	ID          string
	Label       string
	Description string
	Build       func() config.Agent
}

// Choices returns the built-in agent options.
func Choices() []Choice {
	return []Choice{
		{
			ID:          "opencode",
			Label:       "OpenCode",
			Description: "OpenCode TUI on the workspace (credentials from your laptop OpenCode or OPENCODE_API_KEY)",
			Build: func() config.Agent {
				return config.Agent{
					Name:  "opencode",
					Label: "OpenCode",
					Install: []string{
						`command -v opencode >/dev/null 2>&1 || curl -fsSL https://opencode.ai/install | bash`,
						`grep -q '.opencode/bin' "$HOME/.bashrc" 2>/dev/null || echo 'export PATH="$HOME/.opencode/bin:$PATH"' >> "$HOME/.bashrc"`,
					},
					Run:        "exec opencode",
					Credential: "opencode",
				}
			},
		},
		{
			ID:          "claude",
			Label:       "Claude Code",
			Description: "Anthropic Claude Code CLI on the workspace (ANTHROPIC_API_KEY on your laptop)",
			Build: func() config.Agent {
				return config.Agent{
					Name:  "claude",
					Label: "Claude Code",
					Install: []string{
						`command -v node >/dev/null 2>&1 || curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - && sudo apt-get install -y -qq nodejs`,
						`command -v claude >/dev/null 2>&1 || npm install -g @anthropic-ai/claude-code`,
					},
					Run:        "exec claude",
					Credential: "anthropic",
				}
			},
		},
		{
			ID:          "codex",
			Label:       "OpenAI Codex",
			Description: "OpenAI Codex CLI on the workspace (OPENAI_API_KEY on your laptop; ChatGPT sign-in needs an interactive first run)",
			Build: func() config.Agent {
				return config.Agent{
					Name:  "codex",
					Label: "OpenAI Codex",
					Install: []string{
						`command -v codex >/dev/null 2>&1 || CODEX_NON_INTERACTIVE=true curl -fsSL https://chatgpt.com/codex/install.sh | sh`,
					},
					Run:        "exec codex",
					Credential: "openai",
				}
			},
		},
		{
			ID:          "cursor",
			Label:       "Cursor Agent",
			Description: "Cursor Agent CLI on the workspace (CURSOR_API_KEY on your laptop; installs via cursor.com/install)",
			Build: func() config.Agent {
				return config.Agent{
					Name:  "cursor",
					Label: "Cursor Agent",
					Install: []string{
						`command -v agent >/dev/null 2>&1 || curl -fsSL https://cursor.com/install | bash`,
						`grep -q '.local/bin' "$HOME/.bashrc" 2>/dev/null || echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc"`,
					},
					Run:        "exec agent --trust",
					Credential: "cursor",
				}
			},
		},
	}
}

// BuildConfig assembles the global config from wizard answers.
func BuildConfig(agentID string, nvim, gh, ripgrep bool, dotfiles []string, tools []string) *config.Config {
	var agent config.Agent
	for _, c := range Choices() {
		if c.ID == agentID {
			agent = c.Build()
			break
		}
	}

	c := &config.Config{
		SetupVersion: config.CurrentSetupVersion,
		DefaultAgent: agentID,
		Agents:       []config.Agent{agent},
		SpritePrefix: "portable",
		SyncIgnores: []string{
			".git", ".portable", ".stfolder", ".stignore", ".stversions",
			"node_modules", "target", "dist", "build",
			".venv", "venv", "__pycache__", "*.log",
		},
		Dotfiles:    dotfiles,
		Tools:       tools,
		KeepOnError: false,
		Provision: config.Provision{
			Nvim:    nvim,
			Gh:      gh,
			Ripgrep: ripgrep || true,
		},
	}

	switch agent.Credential {
	case "opencode":
		c.EnvFiles = []string{".config/opencode/env"}
	case "anthropic", "openai", "cursor":
		c.EnvFiles = []string{".config/portable/agent.env"}
	}
	return c
}

// ResolveCredentials fills remote credential files from the laptop environment.
func ResolveCredentials(agent config.Agent) (remoteEnvPath, envContent string, warn string) {
	switch agent.Credential {
	case "opencode":
		return ".config/opencode/env", opencodeEnvContent(), opencodeWarn()
	case "anthropic":
		key := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
		if key == "" {
			return ".config/portable/agent.env", "", "ANTHROPIC_API_KEY is not set; set it on your laptop and run portable up --reprovision"
		}
		body := "export ANTHROPIC_API_KEY=" + shellQuote(key) + "\n"
		return ".config/portable/agent.env", body, ""
	case "openai":
		key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		if key == "" {
			return ".config/portable/agent.env", "", "OPENAI_API_KEY is not set; set it on your laptop and run portable up --reprovision (or sign in interactively via portable agent on the workspace)"
		}
		body := "export OPENAI_API_KEY=" + shellQuote(key) + "\n"
		return ".config/portable/agent.env", body, ""
	case "cursor":
		key := strings.TrimSpace(os.Getenv("CURSOR_API_KEY"))
		if key == "" {
			return ".config/portable/agent.env", "", "CURSOR_API_KEY is not set; set it on your laptop and run portable up --reprovision"
		}
		body := "export CURSOR_API_KEY=" + shellQuote(key) + "\n"
		return ".config/portable/agent.env", body, ""
	case "none":
		return "", "", ""
	default:
		return "", "", ""
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
