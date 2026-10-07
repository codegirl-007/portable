package state

import (
	"testing"
	"time"
)

func TestFindByWorkspace(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	inst := Instance{
		ProjectPath: "/tmp/my-app",
		ProjectSlug: "my-app-deadbeef",
		SpriteName:  "portable-my-app-deadbeef",
		SyncSession: "portable-my-app-deadbeef",
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
