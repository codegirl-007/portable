// Package config loads portable's global configuration plus per-project
// overrides.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config is the resolved configuration for a run.
type Config struct {
	SpritePrefix   string   `mapstructure:"sprite_prefix"`
	SyncIgnores    []string `mapstructure:"sync_ignores"`
	Dotfiles       []string `mapstructure:"dotfiles"`
	Tools          []string `mapstructure:"tools"`
	PackageManager string   `mapstructure:"package_manager"`
	KeepOnError    bool     `mapstructure:"keep_on_error"`
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("sprite_prefix", "portable")
	v.SetDefault("sync_ignores", []string{
		".git", ".portable", ".stfolder", ".stignore", ".stversions",
		"node_modules", "target", "dist", "build",
		".venv", "venv", "__pycache__", "*.log",
	})
	v.SetDefault("dotfiles", []string{".config/nvim", ".tmux.conf", ".gitconfig"})
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

	if projectDir != "" {
		for _, name := range []string{".portable.yaml", ".portable.yml"} {
			p := filepath.Join(projectDir, name)
			if _, err := os.Stat(p); err != nil {
				continue
			}
			v.SetConfigFile(p)
			if err := v.MergeInConfig(); err != nil {
				return nil, fmt.Errorf("merge %s: %w", p, err)
			}
		}
	}

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	return &c, nil
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
