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
	"github.com/codegirl-007/portable/internal/gitrepo"
)

// Instance describes one workspace (one Sprite) for a project branch.
type Instance struct {
	ProjectPath string    `json:"project_path"`
	ProjectSlug string    `json:"project_slug"`
	Branch      string    `json:"branch,omitempty"`
	SpriteName  string    `json:"sprite_name"`
	SSHHost     string    `json:"ssh_host"`
	LocalDir    string    `json:"local_dir"`
	RemoteDir   string    `json:"remote_dir"`
	SyncSession string    `json:"sync_session"`
	CreatedAt   time.Time `json:"created_at"`
}

type fileV2 struct {
	Branches map[string]Instance `json:"branches"`
}

// Dir is the per-project state directory (always the canonical repo).
func Dir(projectPath string) string { return filepath.Join(projectPath, ".portable") }

func statePath(projectPath string) string { return filepath.Join(Dir(projectPath), "state.json") }

// LoadAll returns every branch instance for the project.
func LoadAll(projectPath string) (map[string]Instance, error) {
	b, err := os.ReadFile(statePath(projectPath))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Instance{}, nil
	}
	if err != nil {
		return nil, err
	}
	var v2 fileV2
	if err := json.Unmarshal(b, &v2); err == nil && v2.Branches != nil {
		return v2.Branches, nil
	}
	var in Instance
	if err := json.Unmarshal(b, &in); err != nil {
		return nil, fmt.Errorf("parse %s: %w", statePath(projectPath), err)
	}
	if in.SpriteName == "" {
		return map[string]Instance{}, nil
	}
	return map[string]Instance{in.Branch: in}, nil
}

// Load returns the instance for branch, or (nil, nil) when none is recorded.
// A legacy single-object state.json (no branch) is returned for any lookup
// until the next Save migrates it onto the current branch key.
func Load(projectPath, branch string) (*Instance, error) {
	all, err := LoadAll(projectPath)
	if err != nil {
		return nil, err
	}
	if inst, ok := all[branch]; ok {
		copy := inst
		return &copy, nil
	}
	if len(all) == 1 {
		for k, inst := range all {
			if k == "" || inst.Branch == "" {
				copy := inst
				return &copy, nil
			}
		}
	}
	return nil, nil
}

// Save upserts an instance under its branch key, creating .portable/ as needed.
func Save(projectPath string, in *Instance) error {
	if err := os.MkdirAll(Dir(projectPath), 0o700); err != nil {
		return err
	}
	ignore := filepath.Join(Dir(projectPath), ".gitignore")
	if _, err := os.Stat(ignore); errors.Is(err, os.ErrNotExist) {
		_ = os.WriteFile(ignore, []byte("*\n"), 0o600)
	}
	all, err := LoadAll(projectPath)
	if err != nil {
		return err
	}
	if all == nil {
		all = map[string]Instance{}
	}
	in.ProjectPath = projectPath
	if in.ProjectSlug == "" {
		in.ProjectSlug = Slug(projectPath)
	}
	if in.Branch != "" {
		delete(all, "")
	}
	all[in.Branch] = *in
	return writeAll(projectPath, all)
}

func writeAll(projectPath string, all map[string]Instance) error {
	b, err := json.MarshalIndent(fileV2{Branches: all}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(projectPath), append(b, '\n'), 0o600)
}

// Remove deletes one branch's instance. The state file is removed when none remain.
func Remove(projectPath, branch string) error {
	all, err := LoadAll(projectPath)
	if err != nil {
		return err
	}
	delete(all, branch)
	if branch != "" {
		delete(all, "")
	}
	if len(all) == 0 {
		err := os.Remove(statePath(projectPath))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return writeAll(projectPath, all)
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

// BranchSlug is a tag-safe identifier for a git branch (delegates to gitrepo).
func BranchSlug(branch string) string {
	if branch == "" {
		return ""
	}
	return gitrepo.BranchSlug(branch)
}

// WorkspaceName is the Sprite / Mutagen / SSH identity for a project branch.
func WorkspaceName(prefix, projectSlug, branch string) string {
	if prefix == "" {
		prefix = "portable"
	}
	name := prefix + "-" + projectSlug
	if branch != "" {
		name += "-" + BranchSlug(branch)
	}
	return name
}

// --- global registry -------------------------------------------------------

func registryPath() string { return filepath.Join(config.StateDir(), "instances.json") }

// LoadRegistry returns all known instances keyed by sprite name.
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
	migrated := map[string]Instance{}
	changed := false
	for k, in := range out {
		key := in.SpriteName
		if key == "" {
			key = k
		}
		if k != key {
			changed = true
		}
		migrated[key] = in
	}
	if changed {
		_ = saveRegistry(migrated)
	}
	return migrated, nil
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

// Register upserts an instance into the global registry (keyed by sprite name).
func Register(in *Instance) error {
	m, err := LoadRegistry()
	if err != nil {
		return err
	}
	m[in.SpriteName] = *in
	return saveRegistry(m)
}

// Unregister removes a workspace from the global registry by sprite name.
func Unregister(spriteName string) error {
	m, err := LoadRegistry()
	if err != nil {
		return err
	}
	delete(m, spriteName)
	return saveRegistry(m)
}

// FindByWorkspace looks up a registered instance by sprite name, sync session,
// or project slug (as shown in portable ls / portable list).
func FindByWorkspace(id string) (*Instance, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("workspace name is empty")
	}
	m, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	var slugMatch *Instance
	for _, in := range m {
		if in.SpriteName == id || in.SyncSession == id {
			copy := in
			return &copy, nil
		}
		if in.ProjectSlug == id {
			copy := in
			slugMatch = &copy
		}
	}
	if slugMatch != nil {
		return slugMatch, nil
	}
	return nil, fmt.Errorf("unknown workspace %q (see `portable list`)", id)
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
