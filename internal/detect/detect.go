// Package detect inspects a project directory and decides how to install its
// dependencies — notably which JavaScript package manager (npm/pnpm/yarn/bun/
// deno) a project uses.
package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Stack describes what a project needs.
type Stack struct {
	PackageManager        string // npm | pnpm | yarn | bun | deno | ""
	PackageManagerVersion string // from package.json "packageManager"
	NodeVersion           string // engines.node / .nvmrc / .node-version
	Monorepo              bool
	Languages             []string // javascript, typescript, go, rust, python
	Signals               []string // human-readable reasons
}

// Detect inspects dir (no execution) and returns the detected stack.
func Detect(dir string) Stack {
	var s Stack

	hasPkg := false
	if b, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		hasPkg = true
		var pkg struct {
			PackageManager string `json:"packageManager"`
			Engines        struct {
				Node string `json:"node"`
			} `json:"engines"`
			Workspaces json.RawMessage `json:"workspaces"`
		}
		if json.Unmarshal(b, &pkg) == nil {
			if pkg.PackageManager != "" {
				s.PackageManager, s.PackageManagerVersion = splitPM(pkg.PackageManager)
				s.Signals = append(s.Signals, "package.json packageManager="+pkg.PackageManager)
			}
			s.NodeVersion = pkg.Engines.Node
			if len(pkg.Workspaces) > 0 {
				s.Monorepo = true
			}
		}
		s.Languages = append(s.Languages, "javascript")
	}
	if exists(dir, "tsconfig.json") {
		s.Languages = append(s.Languages, "typescript")
	}

	// Lockfiles and config files, in priority order. Deno wins over Bun over
	// pnpm over yarn over npm, because a Deno project may also have package.json.
	if s.PackageManager == "" {
		switch {
		case existsAny(dir, "deno.json", "deno.jsonc", "deno.lock", "import_map.json"):
			s.PackageManager = "deno"
		case existsAny(dir, "bun.lockb", "bun.lock", "bunfig.toml"):
			s.PackageManager = "bun"
		case existsAny(dir, "pnpm-lock.yaml", "pnpm-workspace.yaml"):
			s.PackageManager = "pnpm"
		case existsAny(dir, "yarn.lock", ".yarnrc.yml"):
			s.PackageManager = "yarn"
		case exists(dir, "package-lock.json"):
			s.PackageManager = "npm"
		case hasPkg:
			s.PackageManager = "npm"
		}
		if s.PackageManager != "" {
			s.Signals = append(s.Signals, "lockfile/config for "+s.PackageManager)
		}
	}

	if existsAny(dir, "pnpm-workspace.yaml", "turbo.json", "nx.json", "lerna.json") {
		s.Monorepo = true
	}

	if s.NodeVersion == "" {
		for _, f := range []string{".nvmrc", ".node-version"} {
			if b, err := os.ReadFile(filepath.Join(dir, f)); err == nil {
				if v := strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0]); v != "" {
					s.NodeVersion = strings.TrimPrefix(v, "v")
					break
				}
			}
		}
	}

	if exists(dir, "go.mod") {
		s.Languages = append(s.Languages, "go")
	}
	if exists(dir, "Cargo.toml") {
		s.Languages = append(s.Languages, "rust")
	}
	if existsAny(dir, "pyproject.toml", "requirements.txt") {
		s.Languages = append(s.Languages, "python")
	}

	return s
}

// EnsureToolCommands returns commands that make the package manager available
// on the workspace (idempotent).
func (s Stack) EnsureToolCommands() []string {
	switch s.PackageManager {
	case "bun":
		return []string{`command -v bun >/dev/null 2>&1 || curl -fsSL https://bun.sh/install | bash`}
	case "deno":
		return []string{`command -v deno >/dev/null 2>&1 || curl -fsSL https://deno.land/install.sh | sh`}
	case "pnpm", "yarn":
		pm := s.PackageManager
		return []string{`command -v ` + pm + ` >/dev/null 2>&1 || corepack enable >/dev/null 2>&1 || npm i -g ` + pm}
	}
	return nil
}

// InstallCommands returns the dependency-install command(s).
func (s Stack) InstallCommands() []string {
	switch s.PackageManager {
	case "bun":
		return []string{"bun install"}
	case "deno":
		return []string{"deno install"}
	case "pnpm":
		return []string{"pnpm install"}
	case "yarn":
		return []string{"yarn install"}
	case "npm":
		return []string{"if [ -f package-lock.json ]; then npm ci; else npm install; fi"}
	}
	return nil
}

// Summary is a short human description, e.g. "bun 1.2.0, Node 20, monorepo".
func (s Stack) Summary() string {
	var parts []string
	if s.PackageManager != "" {
		pm := s.PackageManager
		if s.PackageManagerVersion != "" {
			pm += " " + s.PackageManagerVersion
		}
		parts = append(parts, pm)
	}
	if s.NodeVersion != "" {
		parts = append(parts, "Node "+s.NodeVersion)
	}
	if s.Monorepo {
		parts = append(parts, "monorepo")
	}
	return strings.Join(parts, ", ")
}

func splitPM(s string) (name, version string) {
	name = s
	if i := strings.Index(s, "@"); i > 0 {
		name, version = s[:i], s[i+1:]
	}
	if i := strings.Index(version, "+"); i > 0 {
		version = version[:i]
	}
	return name, version
}

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func existsAny(dir string, names ...string) bool {
	for _, n := range names {
		if exists(dir, n) {
			return true
		}
	}
	return false
}
