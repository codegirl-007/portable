// Package mutagen wraps the `mutagen` CLI to keep a local directory and a
// Sprite's directory continuously in sync over SSH.
package mutagen

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// EnsureInstalled reports whether the mutagen CLI is available.
func EnsureInstalled() error {
	if _, err := exec.LookPath("mutagen"); err != nil {
		return fmt.Errorf("mutagen not found; install it from https://github.com/mutagen-io/mutagen/releases (mutagen + mutagen-agents.tar.gz in ~/.local/libexec)")
	}
	return nil
}

func run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "mutagen", args...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if err != nil {
		return out.String(), fmt.Errorf("mutagen %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(out.String()))
	}
	return out.String(), nil
}

// EnsureIgnores adds .worktrees to a Mutagen ignore list if it is missing.
func EnsureIgnores(ignores []string) []string {
	for _, ig := range ignores {
		if ig == ".worktrees" || ig == ".worktrees/" {
			return ignores
		}
	}
	out := make([]string, len(ignores)+1)
	copy(out, ignores)
	out[len(ignores)] = ".worktrees"
	return out
}

// Create starts a two-way sync session between local and host:remotePath.
func Create(ctx context.Context, name, local, host, remotePath string, ignores []string) error {
	ignores = EnsureIgnores(ignores)
	args := []string{"sync", "create", "--name", name, "--ignore-vcs"}
	for _, ig := range ignores {
		args = append(args, "--ignore", ig)
	}
	args = append(args, local, host+":"+remotePath)
	_, err := run(ctx, args...)
	return err
}

// Session is a minimal view of a sync session.
type Session struct {
	Name   string
	Status string
}

// List returns current sync sessions.
func List(ctx context.Context) ([]Session, error) {
	out, err := run(ctx, "sync", "list")
	if err != nil {
		return nil, err
	}
	return parseSessionList(out), nil
}

func parseSessionList(out string) []Session {
	var sessions []Session
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Name:") {
			sessions = append(sessions, Session{Name: strings.TrimSpace(strings.TrimPrefix(line, "Name:"))})
		} else if strings.HasPrefix(line, "Status:") && len(sessions) > 0 {
			sessions[len(sessions)-1].Status = strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
		}
	}
	return sessions
}

// StatusMap returns sync session name to status text.
func StatusMap(ctx context.Context) (map[string]string, error) {
	sessions, err := List(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(sessions))
	for _, s := range sessions {
		m[s.Name] = s.Status
	}
	return m, nil
}

// Halted reports whether a mutagen status is a safety halt (e.g. one-sided
// root emptying) rather than an active or paused session.
func Halted(status string) bool {
	return strings.Contains(status, "Halted")
}

// SyncRunning reports whether a mutagen status means the workspace is up (sync active).
func SyncRunning(status string) bool {
	if status == "" || Halted(status) {
		return false
	}
	return !strings.Contains(status, "Paused")
}

// Lookup returns the named session, or (nil, nil) if it does not exist.
func Lookup(ctx context.Context, name string) (*Session, error) {
	sessions, err := List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range sessions {
		if sessions[i].Name == name {
			s := sessions[i]
			return &s, nil
		}
	}
	return nil, nil
}

// Exists reports whether a session with the given name exists.
func Exists(ctx context.Context, name string) (bool, error) {
	sessions, err := List(ctx)
	if err != nil {
		return false, err
	}
	for _, s := range sessions {
		if s.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// Pause pauses a session (closing the connection so the Sprite can sleep).
func Pause(ctx context.Context, name string) error {
	_, err := run(ctx, "sync", "pause", name)
	return err
}

// Resume resumes a paused session.
func Resume(ctx context.Context, name string) error {
	_, err := run(ctx, "sync", "resume", name)
	return err
}

// Flush waits until a session is fully synchronized.
func Flush(ctx context.Context, name string) error {
	_, err := run(ctx, "sync", "flush", name)
	return err
}

// Terminate removes a session.
func Terminate(ctx context.Context, name string) error {
	_, err := run(ctx, "sync", "terminate", name)
	return err
}

// Recreate terminates an existing session (if any) and creates a new one.
// Use this after Mutagen halts because one side looks empty, which happens
// when a Sprite is new or was recreated and the old session still exists.
func Recreate(ctx context.Context, name, local, host, remotePath string, ignores []string) error {
	_ = Terminate(ctx, name)
	return Create(ctx, name, local, host, remotePath, ignores)
}
