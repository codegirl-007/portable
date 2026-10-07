// Package state persists workspace metadata: a per-project .portable/state.json
// and a global registry.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/codegirl-007/portable/internal/config"
)

// Instance describes one workspace (one Sprite) for a project.
type Instance struct {
	ProjectPath string    `json:"project_path"`
	ProjectSlug string    `json:"project_slug"`
	SpriteName  string    `json:"sprite_name"`
	SSHHost     string    `json:"ssh_host"`
	LocalDir    string    `json:"local_dir"`
	RemoteDir   string    `json:"remote_dir"`
	SyncSession string    `json:"sync_session"`
	CreatedAt   time.Time `json:"created_at"`
}

// Dir is the per-project state directory.
func Dir(projectPath string) string { return filepath.Join(projectPath, ".portable") }

func statePath(projectPath string) string { return filepath.Join(Dir(projectPath), "state.json") }

// Load returns the project's instance, or (nil, nil) when none is recorded.
func Load(projectPath string) (*Instance, error) {
	b, err := os.ReadFile(statePath(projectPath))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var in Instance
	if err := json.Unmarshal(b, &in); err != nil {
		return nil, fmt.Errorf("parse %s: %w", statePath(projectPath), err)
	}
	return &in, nil
}

// Save writes the project's instance, creating .portable/ as needed.
func Save(projectPath string, in *Instance) error {
	if err := os.MkdirAll(Dir(projectPath), 0o700); err != nil {
		return err
	}
	ignore := filepath.Join(Dir(projectPath), ".gitignore")
	if _, err := os.Stat(ignore); errors.Is(err, os.ErrNotExist) {
		_ = os.WriteFile(ignore, []byte("*\n"), 0o600)
	}
	in.ProjectPath = projectPath
	if in.ProjectSlug == "" {
		in.ProjectSlug = Slug(projectPath)
	}
	b, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(projectPath), append(b, '\n'), 0o600)
}

// Remove deletes the project's state file.
func Remove(projectPath string) error {
	err := os.Remove(statePath(projectPath))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Slug is a stable, tag-safe identifier for a project directory.
func Slug(projectPath string) string {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		abs = projectPath
	}
	base := strings.ToLower(filepath.Base(abs))
	base = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "project"
	}
	if len(base) > 24 {
		base = base[:24]
	}
	sum := sha256.Sum256([]byte(abs))
	return base + "-" + hex.EncodeToString(sum[:4])
}

// --- global registry -------------------------------------------------------

func registryPath() string { return filepath.Join(config.StateDir(), "instances.json") }

// LoadRegistry returns all known instances keyed by project path.
func LoadRegistry() (map[string]Instance, error) {
	b, err := os.ReadFile(registryPath())
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Instance{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]Instance{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("parse %s: %w", registryPath(), err)
	}
	return out, nil
}

func saveRegistry(m map[string]Instance) error {
	if err := os.MkdirAll(config.StateDir(), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(registryPath(), append(b, '\n'), 0o600)
}

// Register upserts an instance into the global registry.
func Register(in *Instance) error {
	m, err := LoadRegistry()
	if err != nil {
		return err
	}
	m[in.ProjectPath] = *in
	return saveRegistry(m)
}

// Unregister removes a project from the global registry.
func Unregister(projectPath string) error {
	m, err := LoadRegistry()
	if err != nil {
		return err
	}
	delete(m, projectPath)
	return saveRegistry(m)
}

// Sorted returns the registry values ordered by creation time.
func Sorted(m map[string]Instance) []Instance {
	out := make([]Instance, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}
