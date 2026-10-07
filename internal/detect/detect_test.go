package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectPackageManager(t *testing.T) {
	cases := []struct {
		name    string
		files   []string
		pkgJSON string
		want    string
	}{
		{"npm-lock", []string{"package-lock.json"}, `{}`, "npm"},
		{"yarn", []string{"yarn.lock"}, `{}`, "yarn"},
		{"pnpm", []string{"pnpm-lock.yaml"}, `{}`, "pnpm"},
		{"bun-lockb", []string{"bun.lockb"}, `{}`, "bun"},
		{"bunfig", []string{"bunfig.toml"}, `{}`, "bun"},
		{"deno-json", []string{"deno.json"}, "", "deno"},
		{"deno-beats-pkg", []string{"deno.json"}, `{}`, "deno"},
		{"packageManager-field", nil, `{"packageManager":"pnpm@9.0.0+sha256.abc"}`, "pnpm"},
		{"packageManager-bun", nil, `{"packageManager":"bun@1.2.0"}`, "bun"},
		{"pkg-no-lock", nil, `{}`, "npm"},
		{"none", nil, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range c.files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if c.pkgJSON != "" {
				if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(c.pkgJSON), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := Detect(dir).PackageManager; got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestInstallCommands(t *testing.T) {
	cases := map[string]string{
		"bun":  "bun install",
		"deno": "deno install",
		"pnpm": "pnpm install",
		"yarn": "yarn install",
	}
	for pm, want := range cases {
		got := (Stack{PackageManager: pm}).InstallCommands()
		if len(got) != 1 || got[0] != want {
			t.Errorf("%s: got %v, want [%s]", pm, got, want)
		}
	}
	npm := (Stack{PackageManager: "npm"}).InstallCommands()
	if len(npm) != 1 || !strings.Contains(npm[0], "npm ci") {
		t.Errorf("npm: got %v", npm)
	}
	if got := (Stack{}).InstallCommands(); got != nil {
		t.Errorf("empty: got %v", got)
	}
}

func TestNodeVersion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".nvmrc"), []byte("v20.11.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Detect(dir).NodeVersion; got != "20.11.0" {
		t.Errorf("got %q, want 20.11.0", got)
	}
}

func TestSummaryAndMonorepo(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"packageManager":"bun@1.2.0","engines":{"node":">=20"}}`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "turbo.json"), []byte("{}"), 0o644)
	s := Detect(dir)
	if s.PackageManager != "bun" || s.PackageManagerVersion != "1.2.0" {
		t.Fatalf("pm: %+v", s)
	}
	if !s.Monorepo {
		t.Errorf("expected monorepo")
	}
	if !strings.Contains(s.Summary(), "bun 1.2.0") {
		t.Errorf("summary: %q", s.Summary())
	}
}
