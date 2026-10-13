// Package gitrepo discovers a git checkout and manages portable worktrees.
package gitrepo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const worktreesDir = ".worktrees"

// Repo is a git checkout (main tree or linked worktree).
type Repo struct {
	Worktree  string // this checkout
	Canonical string // main worktree; .portable and .worktrees live here
	CommonDir string // shared .git directory
}

// Discover returns the git repo containing dir, or (nil, nil) if dir is not in one.
func Discover(dir string) (*Repo, error) {
	inside, err := gitOutput(dir, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		return nil, nil
	}
	worktree, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	worktree = filepath.Clean(strings.TrimSpace(worktree))
	common, err := gitOutput(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, err
	}
	common = strings.TrimSpace(common)
	if !filepath.IsAbs(common) {
		common = filepath.Join(worktree, common)
	}
	common = filepath.Clean(common)
	canonical := filepath.Dir(common)
	return &Repo{Worktree: worktree, Canonical: canonical, CommonDir: common}, nil
}

// CurrentBranch is the checked-out branch of this worktree ("HEAD" if detached).
func (r *Repo) CurrentBranch() (string, error) {
	out, err := gitOutput(r.Worktree, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// BranchExists reports whether a local branch ref exists.
func (r *Repo) BranchExists(branch string) (bool, error) {
	branch = normalizeBranch(branch)
	_, err := gitOutput(r.Canonical, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err == nil {
		return true, nil
	}
	return false, nil
}

// WorktreeForBranch returns the checkout path for branch, if any (including the main tree).
func (r *Repo) WorktreeForBranch(branch string) (string, bool, error) {
	branch = normalizeBranch(branch)
	list, err := r.listWorktrees()
	if err != nil {
		return "", false, err
	}
	want := "refs/heads/" + branch
	for _, wt := range list {
		if wt.Branch == want {
			return wt.Path, true, nil
		}
	}
	return "", false, nil
}

// EnsureWorktree returns an existing checkout for branch, or adds one under
// .worktrees/. It does not change the current directory's HEAD.
func (r *Repo) EnsureWorktree(branch string) (string, error) {
	branch = normalizeBranch(branch)
	if branch == "" || branch == "HEAD" {
		return "", fmt.Errorf("branch name is required")
	}
	if path, ok, err := r.WorktreeForBranch(branch); err != nil {
		return "", err
	} else if ok {
		return path, nil
	}
	ok, err := r.BranchExists(branch)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("branch %q does not exist (create it first)", branch)
	}
	if err := r.excludeWorktrees(); err != nil {
		return "", err
	}
	dest := filepath.Join(r.Canonical, worktreesDir, WorktreeFolder(branch))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if _, err := gitOutput(r.Canonical, "worktree", "add", dest, branch); err != nil {
		return "", err
	}
	return dest, nil
}

type listedWorktree struct {
	Path   string
	Branch string // refs/heads/... or empty if detached
}

func (r *Repo) listWorktrees() ([]listedWorktree, error) {
	out, err := gitOutput(r.Canonical, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var list []listedWorktree
	var cur listedWorktree
	flush := func() {
		if cur.Path != "" {
			list = append(list, cur)
		}
		cur = listedWorktree{}
	}
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			cur.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimSpace(strings.TrimPrefix(line, "branch "))
		case line == "":
			flush()
		}
	}
	flush()
	return list, nil
}

func (r *Repo) excludeWorktrees() error {
	path := filepath.Join(r.CommonDir, "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	s := string(existing)
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == worktreesDir+"/" || strings.TrimSpace(line) == worktreesDir {
			return nil
		}
	}
	var b strings.Builder
	b.WriteString(s)
	if s != "" && !strings.HasSuffix(s, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString("# portable\n")
	b.WriteString(worktreesDir + "/\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func normalizeBranch(branch string) string {
	branch = strings.TrimSpace(branch)
	branch = strings.TrimPrefix(branch, "refs/heads/")
	return branch
}

// BranchSlug is a tag-safe identifier for a git branch. feat/foo and feat-foo
// get different suffixes because the hash covers the full ref.
func BranchSlug(branch string) string {
	return slugify(branch, "branch")
}

// WorktreeFolder is the directory name under .worktrees/ for a branch.
func WorktreeFolder(branch string) string {
	return BranchSlug(branch)
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s, fallback string) string {
	base := strings.ToLower(s)
	base = slugNonAlnum.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = fallback
	}
	if len(base) > 24 {
		base = strings.Trim(base[:24], "-")
		if base == "" {
			base = fallback
		}
	}
	sum := sha256.Sum256([]byte(s))
	return base + "-" + hex.EncodeToString(sum[:4])
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	return out.String(), nil
}
