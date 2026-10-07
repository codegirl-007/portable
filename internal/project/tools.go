// Package project interprets per-project .portable.yaml (tools and install steps).
package project

import (
	"fmt"

	"github.com/codegirl-007/portable/internal/provision"
)

// ToolEntry is one runtime/CLI to install on the workspace before ensure/install.
type ToolEntry struct {
	Command string // binary checked with command -v, e.g. bun, cargo
	Install string // shell run when that binary is missing
}

// Config is the project-local .portable.yaml (not global setup).
type Config struct {
	Tools   []ToolEntry `mapstructure:"-"`
	Ensure  []string    `mapstructure:"ensure"`
	Install []string    `mapstructure:"install"`
}

// Defined reports whether the project declares its own tooling or install steps.
func (c Config) Defined() bool {
	return len(c.Tools) > 0 || len(c.Ensure) > 0 || len(c.Install) > 0
}

func toolEnsureLines(tools []ToolEntry) ([]string, error) {
	var out []string
	for i, t := range tools {
		line, err := provision.GuardedToolInstall(t.Command, t.Install)
		if err != nil {
			return nil, fmt.Errorf("tools[%d]: %w", i, err)
		}
		out = append(out, line)
	}
	return out, nil
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
	toolLines, err := toolEnsureLines(proj.Tools)
	if err != nil {
		return DepsPlan{}, err
	}
	ensure := append(toolLines, proj.Ensure...)
	return DepsPlan{
		Ensure:          ensure,
		Install:         append([]string(nil), proj.Install...),
		AutoLangInstall: false,
	}, nil
}
