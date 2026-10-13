package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/config"
	"github.com/codegirl-007/portable/internal/gitrepo"
	"github.com/codegirl-007/portable/internal/mutagen"
	"github.com/codegirl-007/portable/internal/sshconfig"
	"github.com/codegirl-007/portable/internal/state"
)

var (
	flagConfig  string
	flagVerbose bool
	flagBranch  string
	cfg         *config.Config
	projectDir  string
)

// Version is the build version, set via -ldflags at build time.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "portable",
	Short: "Synced remote workspaces on Fly.io",
	Long: "portable keeps the current project directory in sync with a remote\n" +
		"Linux workspace and runs your coding agent or other heavy commands there.\n" +
		"Sync is continuous and two-way via Mutagen over SSH.\n\n" +
		"Run `portable setup` once, then `portable up` in each project.",
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		projectDir = dir
		c, err := config.Load(projectDir, flagConfig)
		if err != nil {
			return err
		}
		cfg = c
		return nil
	},
}

// Execute runs the CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "config file (default ~/.config/portable/config.toml)")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "stream remote setup and dependency output")
	rootCmd.AddCommand(setupCmd, upCmd, downCmd, destroyCmd, agentCmd, sshCmd, nvimCmd, runCmd, statusCmd, lsCmd, listCmd)
}

func bg() context.Context { return context.Background() }

func bindBranchFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&flagBranch, "branch", "", "git branch workspace (uses or creates a worktree under .worktrees/)")
}

// takeBranchFlag pulls --branch from args for commands that disable flag parsing.
func takeBranchFlag(args []string) (branch string, rest []string) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--branch":
			if i+1 >= len(args) {
				return "", args
			}
			branch = args[i+1]
			i++
		case strings.HasPrefix(a, "--branch="):
			branch = strings.TrimPrefix(a, "--branch=")
		default:
			rest = append(rest, a)
		}
	}
	return branch, rest
}

func canonicalDir() string {
	repo, err := gitrepo.Discover(projectDir)
	if err == nil && repo != nil {
		return repo.Canonical
	}
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		return projectDir
	}
	return abs
}

func currentBranch() (string, error) {
	if flagBranch != "" {
		return strings.TrimSpace(flagBranch), nil
	}
	repo, err := gitrepo.Discover(projectDir)
	if err != nil {
		return "", err
	}
	if repo == nil {
		return "", nil
	}
	return repo.CurrentBranch()
}

// shellQuote single-quotes a string for a remote shell command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// displayRemote shortens the remote home path for display.
func displayRemote(remoteDir string) string {
	return strings.Replace(remoteDir, "/home/sprite/", "~/", 1)
}

// newInstance derives the workspace identity for a local directory and branch.
func newInstance(localDir, branch string) (*state.Instance, error) {
	absLocal, err := filepath.Abs(localDir)
	if err != nil {
		return nil, err
	}
	canon := canonicalDir()
	slug := state.Slug(canon)
	sprite := state.WorkspaceName(cfg.SpritePrefix, slug, branch)
	return &state.Instance{
		ProjectPath: canon,
		ProjectSlug: slug,
		Branch:      branch,
		SpriteName:  sprite,
		SSHHost:     sshconfig.Alias(sprite),
		LocalDir:    absLocal,
		RemoteDir:   "/home/sprite/" + filepath.Base(canon),
		SyncSession: sprite,
		CreatedAt:   time.Now(),
	}, nil
}

// resolveInstance returns the workspace for an optional CLI name, --branch, or cwd.
func resolveInstance(args []string) (*state.Instance, error) {
	if flagBranch != "" {
		if len(args) > 0 {
			return nil, fmt.Errorf("pass --branch or a workspace name, not both")
		}
		return loadInstance()
	}
	if len(args) == 0 {
		return loadInstance()
	}
	if len(args) > 1 {
		return nil, fmt.Errorf("expected at most one workspace name")
	}
	return state.FindByWorkspace(args[0])
}

// loadInstance returns the recorded instance for the current project branch.
func loadInstance() (*state.Instance, error) {
	branch, err := currentBranch()
	if err != nil {
		return nil, err
	}
	canon := canonicalDir()
	inst, err := state.Load(canon, branch)
	if err != nil {
		return nil, err
	}
	if inst == nil || inst.SpriteName == "" {
		if branch != "" {
			return nil, fmt.Errorf("no workspace for %s on %s (run `portable up --branch %s`)", canon, branch, branch)
		}
		return nil, fmt.Errorf("no workspace for %s (run `portable up`)", canon)
	}
	return inst, nil
}

// ensureSync resumes the sync session if it is paused (best effort), so
// agent/ssh/run see current files after a `portable down`.
func ensureSync(inst *state.Instance) {
	if inst.SyncSession == "" {
		return
	}
	_ = mutagen.Resume(bg(), inst.SyncSession)
}
