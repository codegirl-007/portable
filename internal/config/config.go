// Package config loads portable's global configuration plus per-project
// overrides.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"
)

const CurrentSetupVersion = 1

// Agent is a coding agent installed and run on the workspace.
type Agent struct {
	Name       string   `mapstructure:"name" toml:"name"`
	Label      string   `mapstructure:"label" toml:"label"`
	Install    []string `mapstructure:"install" toml:"install"`
	Run        string   `mapstructure:"run" toml:"run"`
	Credential string   `mapstructure:"credential" toml:"credential"`
	LocalOnly  bool     `mapstructure:"local_only" toml:"local_only"`
}

// Provision selects optional one-time workspace packages.
type Provision struct {
	Nvim       bool `mapstructure:"nvim" toml:"nvim"`
	Gh         bool `mapstructure:"gh" toml:"gh"`
	Ripgrep    bool `mapstructure:"ripgrep" toml:"ripgrep"`
	TreeSitter bool `mapstructure:"tree_sitter" toml:"tree_sitter"`
}

// Config is the resolved configuration for a run.
type Config struct {
	SetupVersion int      `mapstructure:"setup_version" toml:"setup_version"`
	DefaultAgent string   `mapstructure:"default_agent" toml:"default_agent"`
	Agents       []Agent  `mapstructure:"agents" toml:"agents"`
	EnvFiles     []string `mapstructure:"env_files" toml:"env_files"`
	Provision    Provision `mapstructure:"provision" toml:"provision"`

	SpritePrefix   string   `mapstructure:"sprite_prefix" toml:"sprite_prefix"`
	SyncIgnores    []string `mapstructure:"sync_ignores" toml:"sync_ignores"`
	Dotfiles       []string `mapstructure:"dotfiles" toml:"dotfiles"`
	// Tools are one-time shell commands run at workspace provision (from setup advanced).
	Tools          []string `mapstructure:"tools" toml:"tools"`
	PackageManager string   `mapstructure:"package_manager" toml:"package_manager"`
	KeepOnError    bool     `mapstructure:"keep_on_error" toml:"keep_on_error"`
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("sprite_prefix", "portable")
	v.SetDefault("sync_ignores", []string{
		".git", ".portable", ".stfolder", ".stignore", ".stversions",
		"node_modules", "target", "dist", "build",
		".venv", "venv", "__pycache__", "*.log",
	})
	v.SetDefault("dotfiles", []string{})
	v.SetDefault("keep_on_error", false)
}

// Load reads the global config (or explicitPath) and merges an optional
// .portable.yaml/.yml from projectDir.
func Load(projectDir, explicitPath string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	if explicitPath != "" {
		v.SetConfigFile(explicitPath)
	} else {
		v.SetConfigFile(GlobalPath())
	}
	if err := v.ReadInConfig(); err != nil {
		if !isNotFound(err) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	return &c, nil
}

// SetupDone reports whether portable setup has been completed on this machine.
func (c *Config) SetupDone() bool {
	return c.SetupVersion >= CurrentSetupVersion && strings.TrimSpace(c.DefaultAgent) != ""
}

// AgentByName returns a configured agent by id.
func (c *Config) AgentByName(name string) (*Agent, bool) {
	name = strings.TrimSpace(name)
	for i := range c.Agents {
		if c.Agents[i].Name == name {
			return &c.Agents[i], true
		}
	}
	return nil, false
}

// DefaultAgentConfig returns the primary agent from setup.
func (c *Config) DefaultAgentConfig() (*Agent, bool) {
	return c.AgentByName(c.DefaultAgent)
}

// RemotePathPrefixes returns extra PATH segments for remote shells.
func (c *Config) RemotePathPrefixes() []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(p string) {
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add("$HOME/.local/bin")
	for _, a := range c.Agents {
		switch a.Name {
		case "opencode":
			add("$HOME/.opencode/bin")
		case "claude":
			add("$HOME/.npm-global/bin")
			add("$HOME/.local/bin")
		}
	}
	return out
}

// RemoteShellPreamble is a shell prefix that sets PATH and sources env_files.
func (c *Config) RemoteShellPreamble() string {
	parts := make([]string, 0, 2+len(c.EnvFiles))
	path := strings.Join(c.RemotePathPrefixes(), ":")
	parts = append(parts, fmt.Sprintf(`export PATH="%s:$PATH"`, path))
	for _, rel := range c.EnvFiles {
		rel = strings.TrimPrefix(rel, "/")
		parts = append(parts, fmt.Sprintf(`if [ -f "$HOME/%s" ]; then set -a; . "$HOME/%s"; set +a; fi`, shellEscape(rel), shellEscape(rel)))
	}
	return strings.Join(parts, "; ") + "; "
}

func shellEscape(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}

// WriteGlobal writes the machine-wide portable config (setup output).
func WriteGlobal(c *Config) error {
	path := GlobalPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		backup := path + ".bak"
		_ = os.Remove(backup)
		if err := os.Rename(path, backup); err != nil {
			return fmt.Errorf("backup existing config: %w", err)
		}
	}
	b, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	header := "# Written by `portable setup`. Re-run setup to change agents and defaults.\n\n"
	return os.WriteFile(path, append([]byte(header), b...), 0o600)
}

func isNotFound(err error) bool {
	var nf viper.ConfigFileNotFoundError
	return errors.As(err, &nf) || errors.Is(err, os.ErrNotExist)
}

// GlobalPath is ~/.config/portable/config.toml (honouring XDG_CONFIG_HOME).
func GlobalPath() string {
	return filepath.Join(configHome(), "portable", "config.toml")
}

// StateDir is ~/.local/state/portable (honouring XDG_STATE_HOME).
func StateDir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "portable")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "portable")
}

func configHome() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}
