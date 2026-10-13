package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindByWorkspace(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	inst := Instance{
		ProjectPath: "/tmp/my-app",
		ProjectSlug: "my-app-deadbeef",
		Branch:      "main",
		SpriteName:  "portable-my-app-deadbeef-main-aaaabbbb",
		SyncSession: "portable-my-app-deadbeef-main-aaaabbbb",
		CreatedAt:   time.Now(),
	}
	if err := Register(&inst); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{inst.SpriteName, inst.SyncSession, inst.ProjectSlug} {
		got, err := FindByWorkspace(id)
		if err != nil || got.SpriteName != inst.SpriteName {
			t.Fatalf("FindByWorkspace(%q) = %v, %v", id, got, err)
		}
	}
	if _, err := FindByWorkspace("missing"); err == nil {
		t.Fatal("expected error for unknown workspace")
	}
}

func TestRegistryKeyedBySpriteName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	a := Instance{ProjectPath: "/tmp/app", SpriteName: "portable-app-aaaa-main-1111", Branch: "main", CreatedAt: time.Now()}
	b := Instance{ProjectPath: "/tmp/app", SpriteName: "portable-app-aaaa-feat-2222", Branch: "feat/foo", CreatedAt: time.Now().Add(time.Second)}
	if err := Register(&a); err != nil {
		t.Fatal(err)
	}
	if err := Register(&b); err != nil {
		t.Fatal(err)
	}
	m, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Fatalf("registry len = %d, want 2", len(m))
	}
	if m[a.SpriteName].Branch != "main" || m[b.SpriteName].Branch != "feat/foo" {
		t.Fatalf("registry = %#v", m)
	}
	if err := Unregister(a.SpriteName); err != nil {
		t.Fatal(err)
	}
	m, _ = LoadRegistry()
	if _, ok := m[b.SpriteName]; !ok || len(m) != 1 {
		t.Fatalf("after unregister = %#v", m)
	}
}

func TestLoadMigratesOldStateJSON(t *testing.T) {
	dir := t.TempDir()
	old := Instance{
		ProjectPath: dir,
		ProjectSlug: "app-deadbeef",
		SpriteName:  "portable-app-deadbeef",
		SyncSession: "portable-app-deadbeef",
		LocalDir:    dir,
		CreatedAt:   time.Now(),
	}
	if err := os.MkdirAll(Dir(dir), 0o700); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(old, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(dir), "state.json"), append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Load(dir, "main")
	if err != nil || got == nil || got.SpriteName != old.SpriteName {
		t.Fatalf("Load(main) = %#v, %v", got, err)
	}

	got.Branch = "main"
	if err := Save(dir, got); err != nil {
		t.Fatal(err)
	}
	all, err := LoadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := all[""]; ok {
		t.Fatalf("empty key still present: %#v", all)
	}
	if all["main"].SpriteName != old.SpriteName {
		t.Fatalf("migrated = %#v", all)
	}
}

func TestSaveTwoBranches(t *testing.T) {
	dir := t.TempDir()
	main := Instance{SpriteName: "portable-app-main", Branch: "main", LocalDir: dir, CreatedAt: time.Now()}
	feat := Instance{SpriteName: "portable-app-feat", Branch: "feat/foo", LocalDir: dir + "/.worktrees/feat", CreatedAt: time.Now()}
	if err := Save(dir, &main); err != nil {
		t.Fatal(err)
	}
	if err := Save(dir, &feat); err != nil {
		t.Fatal(err)
	}
	all, err := LoadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all["main"].SpriteName != main.SpriteName || all["feat/foo"].SpriteName != feat.SpriteName {
		t.Fatalf("LoadAll = %#v", all)
	}
	if err := Remove(dir, "feat/foo"); err != nil {
		t.Fatal(err)
	}
	all, _ = LoadAll(dir)
	if len(all) != 1 || all["main"].SpriteName == "" {
		t.Fatalf("after remove = %#v", all)
	}
}

func TestWorkspaceName(t *testing.T) {
	a := WorkspaceName("portable", "app-deadbeef", "feat/foo")
	b := WorkspaceName("portable", "app-deadbeef", "feat-foo")
	if a == b {
		t.Fatalf("collision: %q", a)
	}
	if WorkspaceName("portable", "app-deadbeef", "") != "portable-app-deadbeef" {
		t.Fatalf("empty branch = %q", WorkspaceName("portable", "app-deadbeef", ""))
	}
}

func TestRegistryMigratesPathKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	legacy := map[string]Instance{
		"/tmp/app": {ProjectPath: "/tmp/app", SpriteName: "portable-app-deadbeef", CreatedAt: time.Now()},
	}
	if err := os.MkdirAll(filepath.Join(dir, "portable"), 0o700); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "portable", "instances.json"), append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m["portable-app-deadbeef"]; !ok {
		t.Fatalf("expected sprite key, got %#v", m)
	}
}
