package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codegirl-007/portable/internal/config"
	"github.com/codegirl-007/portable/internal/gitrepo"
)

func TestTakeBranchFlag(t *testing.T) {
	b, rest := takeBranchFlag([]string{"--branch", "feat/foo", "extra"})
	if b != "feat/foo" || strings.Join(rest, " ") != "extra" {
		t.Fatalf("got %q %q", b, rest)
	}
	b, rest = takeBranchFlag([]string{"--branch=feat/foo", "a", "b"})
	if b != "feat/foo" || len(rest) != 2 {
		t.Fatalf("got %q %#v", b, rest)
	}
	b, rest = takeBranchFlag([]string{"go", "test"})
	if b != "" || len(rest) != 2 {
		t.Fatalf("got %q %#v", b, rest)
	}
}

func TestPrepareUpMissingBranch(t *testing.T) {
	dir := initCmdRepo(t)
	defer resetUpTest()
	projectDir = dir
	flagBranch = "does-not-exist"
	_, err := prepareUpInstance()
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("err = %v", err)
	}
}

func TestPrepareUpNonGitBranch(t *testing.T) {
	defer resetUpTest()
	projectDir = t.TempDir()
	flagBranch = "feat/foo"
	_, err := prepareUpInstance()
	if err == nil || !strings.Contains(err.Error(), "git repository") {
		t.Fatalf("err = %v", err)
	}
}

func TestPrepareUpWorktreeAddAndReuse(t *testing.T) {
	dir := initCmdRepo(t)
	runCmdGit(t, dir, "branch", "feat/foo")
	defer resetUpTest()
	cfg = &config.Config{SpritePrefix: "portable"}
	projectDir = dir
	flagBranch = "feat/foo"

	inst, err := prepareUpInstance()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".worktrees", gitrepo.WorktreeFolder("feat/foo"))
	if inst.LocalDir != want {
		t.Fatalf("LocalDir = %q, want %q", inst.LocalDir, want)
	}
	if inst.Branch != "feat/foo" {
		t.Fatalf("Branch = %q", inst.Branch)
	}
	if !strings.Contains(inst.SpriteName, gitrepo.BranchSlug("feat/foo")) {
		t.Fatalf("SpriteName = %q", inst.SpriteName)
	}

	again, err := prepareUpInstance()
	if err != nil {
		t.Fatal(err)
	}
	if again.LocalDir != inst.LocalDir {
		t.Fatalf("reuse LocalDir = %q", again.LocalDir)
	}
}

func TestPrepareUpUsesCwdWhenAlreadyOnBranch(t *testing.T) {
	dir := initCmdRepo(t)
	defer resetUpTest()
	cfg = &config.Config{SpritePrefix: "portable"}
	projectDir = dir
	repo, err := gitrepo.Discover(dir)
	if err != nil || repo == nil {
		t.Fatalf("Discover: %v %v", repo, err)
	}
	br, err := repo.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	flagBranch = br
	inst, err := prepareUpInstance()
	if err != nil {
		t.Fatal(err)
	}
	if inst.LocalDir != dir {
		t.Fatalf("LocalDir = %q, want main tree %q", inst.LocalDir, dir)
	}
}

func TestTwoBranchesGetDifferentSprites(t *testing.T) {
	dir := initCmdRepo(t)
	runCmdGit(t, dir, "branch", "feat/foo")
	defer resetUpTest()
	cfg = &config.Config{SpritePrefix: "portable"}
	projectDir = dir

	repo, err := gitrepo.Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	current, err := repo.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	flagBranch = current
	a, err := prepareUpInstance()
	if err != nil {
		t.Fatal(err)
	}
	flagBranch = "feat/foo"
	b, err := prepareUpInstance()
	if err != nil {
		t.Fatal(err)
	}
	if a.SpriteName == b.SpriteName || a.SyncSession == b.SyncSession {
		t.Fatalf("expected distinct identities: %q vs %q", a.SpriteName, b.SpriteName)
	}
	if a.LocalDir == b.LocalDir {
		t.Fatalf("expected distinct LocalDir, both %q", a.LocalDir)
	}
}

func resetUpTest() {
	flagBranch = ""
	projectDir = ""
	cfg = nil
}

func initCmdRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runCmdGit(t, dir, "-c", "init.defaultBranch=main", "init")
	runCmdGit(t, dir, "config", "user.email", "test@example.com")
	runCmdGit(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCmdGit(t, dir, "add", "README")
	runCmdGit(t, dir, "commit", "-m", "init")
	return dir
}

func runCmdGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}
