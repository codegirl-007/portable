package setup

import (
	"fmt"

	"github.com/codegirl-007/portable/internal/opencode"
)

func opencodeEnvContent() string {
	key, err := opencode.GoAPIKey()
	if err != nil || key == "" {
		return ""
	}
	model := opencode.DefaultModel()
	var b string
	b += `export PATH="$HOME/.opencode/bin:$HOME/.local/bin:$PATH"` + "\n"
	b += fmt.Sprintf("export OPENCODE_API_KEY=%s\n", shellQuote(key))
	if model != "" {
		b += fmt.Sprintf("# default model copied from laptop: %s\n", model)
	}
	return b
}

func opencodeWarn() string {
	if _, err := opencode.GoAPIKey(); err != nil {
		return "no OpenCode credentials found locally; set OPENCODE_API_KEY or connect OpenCode Go, then portable up --reprovision"
	}
	return ""
}

// OpenCodeModel returns the laptop default model for opencode.jsonc on the workspace.
func OpenCodeModel() string {
	return opencode.DefaultModel()
}
