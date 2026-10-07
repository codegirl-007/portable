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
	for _, t := range c.Tools {
		if strings.TrimSpace(t) == "" {
			return fmt.Errorf("tools: empty tool name")
		}
	}
	return nil
}
