// Package sprites wraps the `sprite` CLI (Fly.io Sprites) for the operations
// portable needs: lifecycle, command execution, and file transfer.
package sprites

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// Client shells out to the sprite CLI.
type Client struct{}

// New returns a Client.
func New() *Client { return &Client{} }

// EnsureInstalled reports whether the sprite CLI is available.
func EnsureInstalled() error {
	if _, err := exec.LookPath("sprite"); err != nil {
		return fmt.Errorf("sprite CLI not found; install it: curl https://sprites.dev/install.sh | bash")
	}
	return nil
}

func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "sprite", args...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if err != nil {
		return out.String(), fmt.Errorf("sprite %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(out.String()))
	}
	return out.String(), nil
}

// List returns the names of existing sprites.
func (c *Client) List(ctx context.Context) ([]string, error) {
	out, err := c.run(ctx, "list")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, " ") {
			names = append(names, line)
		}
	}
	return names, nil
}

// Exists reports whether a sprite with the given name exists.
func (c *Client) Exists(ctx context.Context, name string) (bool, error) {
	names, err := c.List(ctx)
	if err != nil {
		return false, err
	}
	for _, n := range names {
		if n == name {
			return true, nil
		}
	}
	return false, nil
}

// Create creates a sprite (idempotent: a no-op if it already exists).
func (c *Client) Create(ctx context.Context, name string) error {
	if ok, err := c.Exists(ctx, name); err != nil {
		return err
	} else if ok {
		return nil
	}
	_, err := c.run(ctx, "create", name, "--skip-console")
	return err
}

// Destroy permanently deletes a sprite.
func (c *Client) Destroy(ctx context.Context, name string) error {
	_, err := c.run(ctx, "destroy", "-s", name, "--force")
	return err
}

// ExecOptions controls command execution.
type ExecOptions struct {
	Dir    string
	TTY    bool
	Env    []string // KEY=VALUE
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Exec runs a command in the sprite, streaming IO.
func (c *Client) Exec(ctx context.Context, name string, opts ExecOptions, command ...string) error {
	args := []string{"exec", "-s", name}
	if opts.TTY {
		args = append(args, "--tty")
	}
	if opts.Dir != "" {
		args = append(args, "--dir", opts.Dir)
	}
	if len(opts.Env) > 0 {
		args = append(args, "--env", strings.Join(opts.Env, ","))
	}
	args = append(args, "--")
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, "sprite", args...)
	cmd.Stdin = opts.Stdin
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	cmd.Stdout, cmd.Stderr = opts.Stdout, opts.Stderr
	return cmd.Run()
}

// Output runs a command and returns its combined output.
func (c *Client) Output(ctx context.Context, name string, command ...string) (string, error) {
	args := append([]string{"exec", "-s", name, "--no-stdin", "--"}, command...)
	return c.run(ctx, args...)
}

// FilePush copies a local file or directory into the sprite. It streams a tar
// archive over `sprite exec` stdin, which (unlike `sprite file push`) handles
// large files reliably and preserves permissions.
func (c *Client) FilePush(ctx context.Context, name, src, dest string) error {
	parent := filepath.Dir(src)
	base := filepath.Base(src)
	destParent := path.Dir(dest)
	remote := fmt.Sprintf("mkdir -p %s && tar xzf - -C %s", shq(destParent), shq(destParent))

	tarCmd := exec.CommandContext(ctx, "tar", "-C", parent, "-czf", "-", base)
	spCmd := exec.CommandContext(ctx, "sprite", "exec", "-s", name, "--", "sh", "-c", remote)

	pipe, err := tarCmd.StdoutPipe()
	if err != nil {
		return err
	}
	spCmd.Stdin = pipe
	var errBuf bytes.Buffer
	spCmd.Stderr = &errBuf

	if err := tarCmd.Start(); err != nil {
		return err
	}
	if err := spCmd.Start(); err != nil {
		_ = tarCmd.Wait()
		return err
	}
	tarErr := tarCmd.Wait()
	spErr := spCmd.Wait()
	if spErr != nil {
		return fmt.Errorf("push %s: %w: %s", src, spErr, strings.TrimSpace(errBuf.String()))
	}
	if tarErr != nil {
		return fmt.Errorf("tar %s: %w", src, tarErr)
	}
	return nil
}

func shq(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
