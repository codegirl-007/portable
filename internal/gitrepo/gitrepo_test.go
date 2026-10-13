package gitrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchSlugDistinguishesSlashVsHyphen(t *testing.T) {
	a := BranchSlug("feat/foo")
	b := BranchSlug("feat-foo")
	if a == b {
		t.Fatalf("feat/foo and feat-foo both slug to %q", a)
	}
	if !strings.HasPrefix(a, "feat-foo-") || !strings.HasPrefix(b, "feat-foo-") {
		t.Fatalf("slugs = %q, %q", a, b)
	}
}

func TestDiscoverNonGit(t *testing.T) {
	repo, err := Discover(t.TempDir())
	if err != nil || repo != nil {
		t.Fatalf("Discover = %v, %v", repo, err)
	}
}

func TestEnsureWorktreeAddReuseAndMissing(t *testing.T) {
	dir := initRepo(t)
	repo, err := Discover(dir)
	if err != nil || repo == nil {
		t.Fatalf("Discover: %v %v", repo, err)
	}
	if repo.Canonical != dir {
		t.Fatalf("Canonical = %q, want %q", repo.Canonical, dir)
	}

	if _, err := repo.EnsureWorktree("does-not-exist"); err == nil {
		t.Fatal("expected error for missing branch")
	}

	runGit(t, dir, "branch", "feat/foo")
	path, err := repo.EnsureWorktree("feat/foo")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".worktrees", WorktreeFolder("feat/foo"))
	if path != want {
		t.Fatalf("worktree path = %q, want %q", path, want)
	}
	if _, err := os.Stat(filepath.Join(path, "README")); err != nil {
		t.Fatal(err)
	}

	again, err := repo.EnsureWorktree("feat/foo")
	if err != nil {
		t.Fatal(err)
	}
	if again != path {
		t.Fatalf("reuse = %q, want %q", again, path)
	}

	exclude := filepath.Join(repo.CommonDir, "info", "exclude")
	b, err := os.ReadFile(exclude)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), ".worktrees/") {
		t.Fatalf("exclude = %q", b)
	}

	// Branch already checked out in the main tree is reused, not duplicated.
	current, err := repo.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	main, err := repo.EnsureWorktree(current)
	if err != nil {
		t.Fatal(err)
	}
	if main != dir {
		t.Fatalf("main worktree = %q, want %q", main, dir)
	}
}

func TestDiscoverFromLinkedWorktree(t *testing.T) {
	dir := initRepo(t)
	runGit(t, dir, "branch", "feat/bar")
	repo, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.EnsureWorktree("feat/bar")
	if err != nil {
		t.Fatal(err)
	}
	fromWT, err := Discover(wt)
	if err != nil || fromWT == nil {
		t.Fatalf("Discover(worktree) = %v, %v", fromWT, err)
	}
	if fromWT.Canonical != dir {
		t.Fatalf("Canonical from worktree = %q, want %q", fromWT.Canonical, dir)
	}
	br, err := fromWT.CurrentBranch()
	if err != nil || br != "feat/bar" {
		t.Fatalf("CurrentBranch = %q, %v", br, err)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "-c", "init.defaultBranch=main", "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}
