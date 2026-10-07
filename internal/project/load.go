package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
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
		return c, nil
	}
	return c, nil
}
