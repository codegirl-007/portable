package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/codegirl-007/portable/internal/provision"
)

// Load reads .portable.yaml or .portable.yml from projectDir, if present.
func Load(projectDir string) (Config, error) {
	var c Config
	for _, name := range []string{".portable.yaml", ".portable.yml"} {
		p := filepath.Join(projectDir, name)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		v := viper.New()
		v.SetConfigFile(p)
		if err := v.ReadInConfig(); err != nil {
			return c, fmt.Errorf("read %s: %w", name, err)
		}
		if err := v.Unmarshal(&c); err != nil {
			return c, fmt.Errorf("decode %s: %w", name, err)
		}
		tools, err := parseToolEntries(v.Get("tools"))
		if err != nil {
			return c, fmt.Errorf("%s: %w", name, err)
		}
		c.Tools = tools
		if err := validateProjectConfig(&c); err != nil {
			return c, fmt.Errorf("%s: %w", name, err)
		}
		return c, nil
	}
	return c, nil
}

func validateProjectConfig(c *Config) error {
	if err := provision.ValidateShellLines(c.Ensure); err != nil {
		return fmt.Errorf("ensure: %w", err)
	}
	if err := provision.ValidateShellLines(c.Install); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	for i, t := range c.Tools {
		if strings.TrimSpace(t.Command) == "" {
			return fmt.Errorf("tools[%d]: command is required", i)
		}
		if strings.TrimSpace(t.Install) == "" {
			return fmt.Errorf("tools[%d]: install is required", i)
		}
		if err := provision.ValidateShellLines([]string{t.Install}); err != nil {
			return fmt.Errorf("tools[%d]: %w", i, err)
		}
	}
	return nil
}

func parseToolEntries(raw any) ([]ToolEntry, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("tools must be a list")
	}
	var out []ToolEntry
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("tools[%d]: expected command and install fields", i)
		}
		cmd := strings.TrimSpace(stringField(m, "command"))
		install := strings.TrimSpace(stringField(m, "install"))
		if cmd == "" && install == "" {
			return nil, fmt.Errorf("tools[%d]: command and install are required", i)
		}
		out = append(out, ToolEntry{Command: cmd, Install: install})
	}
	return out, nil
}

func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
