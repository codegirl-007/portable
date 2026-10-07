package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/detect"
	"github.com/codegirl-007/portable/internal/mutagen"
	"github.com/codegirl-007/portable/internal/opencode"
	"github.com/codegirl-007/portable/internal/provision"
	"github.com/codegirl-007/portable/internal/sprites"
	"github.com/codegirl-007/portable/internal/sshconfig"
	"github.com/codegirl-007/portable/internal/state"
	"github.com/codegirl-007/portable/internal/ui"
)

var upFlags struct {
	reprovision bool
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Create or wake the project's workspace and sync it",
	RunE:  runUp,
}

func init() {
	upCmd.Flags().BoolVar(&upFlags.reprovision, "reprovision", false, "re-run the one-time setup script (e.g. after adding tools)")
}

func runUp(_ *cobra.Command, _ []string) error {
	if err := sprites.EnsureInstalled(); err != nil {
		return err
	}
	if err := mutagen.EnsureInstalled(); err != nil {
		return err
	}

	inst, err := newInstance()
	if err != nil {
		return err
	}
	if existing, err := state.Load(projectDir); err != nil {
		return err
	} else if existing != nil && existing.SpriteName != "" {
		inst = existing
	}
	if inst.SpriteName == "" {
		return fmt.Errorf("could not determine a workspace name for %s", projectDir)
	}

	client := sprites.New()

	upStep("ensuring workspace")
	if err := client.Create(bg(), inst.SpriteName); err != nil {
		return err
	}
	if err := sshconfig.Ensure(inst.SpriteName); err != nil {
		return err
	}

	provisioned, err := isProvisioned(client, inst.SpriteName)
	if err != nil {
		return err
	}
	if !provisioned || upFlags.reprovision {
		upStep("provisioning the workspace (one-time)")
		goKey, keyErr := opencode.GoAPIKey()
		if keyErr != nil {
			ui.Warnf("%v; the remote opencode will have no credentials", keyErr)
		}
		script := provision.SetupScript(provision.SetupRequest{
			PublicKey: publicKey(),
			GoAPIKey:  goKey,
			Model:     opencode.DefaultModel(),
			RemoteDir: inst.RemoteDir,
			Tools:     cfg.Tools,
		})
		if err := remoteRun(client, inst.SpriteName, sprites.ExecOptions{Stdin: strings.NewReader(script)}, "bash", "-s"); err != nil {
			return failSetup(inst, err)
		}
		pushDotfiles(client, inst.SpriteName)
		upStep("bootstrapping nvim (plugins + treesitter parsers)")
		if err := client.Exec(bg(), inst.SpriteName, sprites.ExecOptions{}, "sh", "-c",
			`export PATH="$HOME/.local/bin:$PATH"; nvim --headless "+Lazy! sync" +qa >/dev/null 2>&1 || true`); err != nil {
			ui.Warnf("nvim bootstrap: %v", err)
		}
	}

	upStep("waiting for workspace connection")
	if err := waitForSSH(inst.SSHHost, 45*time.Second); err != nil {
		return fmt.Errorf("workspace SSH did not become ready: %w", err)
	}

	exists, err := mutagen.Exists(bg(), inst.SyncSession)
	if err != nil {
		return err
	}
	if exists {
		upStep("resuming sync")
		if err := mutagen.Resume(bg(), inst.SyncSession); err != nil {
			return err
		}
	} else {
		upStep("starting sync %s <-> %s", inst.LocalDir, inst.RemoteDir)
		if err := mutagen.Create(bg(), inst.SyncSession, inst.LocalDir, inst.SSHHost, inst.RemoteDir, cfg.SyncIgnores); err != nil {
			return failSetup(inst, err)
		}
	}

	upStep("waiting for the initial sync")
	if err := mutagen.Flush(bg(), inst.SyncSession); err != nil {
		return err
	}

	stack := detect.Detect(inst.LocalDir)
	if flagVerbose {
		if sum := stack.Summary(); sum != "" {
			ui.Infof("detected: %s", sum)
		}
	}
	ensure := stack.EnsureToolCommands()
	install := stack.InstallCommands()
	if cfg.PackageManager != "" {
		stack.PackageManager = cfg.PackageManager
		ensure = stack.EnsureToolCommands()
		install = stack.InstallCommands()
	}

	upStep("installing project dependencies")
	if err := remoteRun(client, inst.SpriteName, sprites.ExecOptions{
		Stdin: strings.NewReader(provision.DepsScript(provision.DepsRequest{
			RemoteDir: inst.RemoteDir,
			Ensure:    ensure,
			Install:   install,
		})),
	}, "bash", "-s"); err != nil {
		ui.Warnf("dependency install failed (continuing): %v", err)
	}

	if err := state.Save(projectDir, inst); err != nil {
		return err
	}
	if err := state.Register(inst); err != nil {
		return err
	}

	ui.Successf("workspace ready")
	if flagVerbose {
		ui.Infof("sync:     %s <-> %s", inst.LocalDir, displayRemote(inst.RemoteDir))
		ui.Infof("shell:    portable ssh")
		ui.Infof("opencode: portable opencode")
		ui.Infof("nvim:     portable nvim")
		ui.Infof("run:      portable run <command>")
		ui.Infof("sleep:    portable down    destroy: portable destroy")
	}
	return nil
}

// waitForSSH waits for the Sprite's sshd service to be ready before Mutagen
// tries to install or resume its remote agent. A Sprite can wake before its
// services have finished starting, causing Mutagen's SSH banner exchange to
// time out.
func waitForSSH(host string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		ctx, cancel := context.WithTimeout(bg(), 4*time.Second)
		cmd := exec.CommandContext(ctx, "ssh",
			"-o", "BatchMode=yes",
			"-o", "ConnectTimeout=3",
			host, "true",
		)
		out, err := cmd.CombinedOutput()
		cancel()
		if err == nil {
			return nil
		}
		lastErr = fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s: %w", timeout, lastErr)
		}
		time.Sleep(time.Second)
	}
}

func isProvisioned(client *sprites.Client, name string) (bool, error) {
	out, err := client.Output(bg(), name, "sh", "-c", "test -f $HOME/.portable-provisioned && echo yes || echo no")
	if err != nil {
		return false, err
	}
	return strings.Contains(out, "yes"), nil
}

// pushDotfiles copies the configured home-relative paths from the laptop to the
// Sprite once, at provision time, preserving executable/secret permissions.
func pushDotfiles(client *sprites.Client, name string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	copied := 0
	for _, rel := range cfg.Dotfiles {
		src := filepath.Join(home, rel)
		info, err := os.Stat(src) // follows symlinks
		if err != nil {
			continue
		}
		real := src
		if r, err := filepath.EvalSymlinks(src); err == nil {
			real = r
		}
		dest := "/home/sprite/" + rel
		if err := client.FilePush(bg(), name, real, dest); err != nil {
			ui.Warnf("dotfiles: %s: %v", rel, err)
			continue
		}
		if !info.IsDir() {
			_, _ = client.Output(bg(), name, "chmod", fmt.Sprintf("%o", info.Mode().Perm()), dest)
		}
		copied++
	}
	if copied > 0 && flagVerbose {
		ui.Infof("copied %d dotfile paths", copied)
	}
}

func publicKey() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, name := range []string{"id_ed25519.pub", "id_rsa.pub"} {
		if b, err := os.ReadFile(filepath.Join(home, ".ssh", name)); err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return ""
}

func failSetup(inst *state.Instance, cause error) error {
	ui.Warnf("setup failed: %v", cause)
	if cfg.KeepOnError {
		ui.Warnf("keeping the workspace (keep_on_error=true)")
		return cause
	}
	ui.Warnf("destroying the workspace (set keep_on_error=true to keep it)")
	_ = mutagen.Terminate(bg(), inst.SyncSession)
	_ = sprites.New().Destroy(bg(), inst.SpriteName)
	_ = sshconfig.Remove(inst.SpriteName)
	_ = state.Remove(projectDir)
	_ = state.Unregister(projectDir)
	return cause
}
