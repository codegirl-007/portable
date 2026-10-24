package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/codegirl-007/portable/internal/remote"
	"github.com/codegirl-007/portable/internal/sprites"
	"github.com/codegirl-007/portable/internal/ui"
)

var remoteFlags struct {
	off        bool
	resetToken bool
	noQR       bool
}

var remoteAccessCmd = &cobra.Command{
	Use:   "remote",
	Short: "Open the agent session from a phone or browser",
	Long: "Starts an authenticated web terminal on the workspace, attached to the\n" +
		"same tmux session as `portable agent`. Prints a URL (and a QR code) to open.",
	Args: cobra.NoArgs,
	RunE: runRemoteAccess,
}

func init() {
	bindBranchFlag(remoteAccessCmd)
	remoteAccessCmd.Flags().BoolVar(&remoteFlags.off, "off", false, "stop the web terminal and make the Sprite URL private again")
	remoteAccessCmd.Flags().BoolVar(&remoteFlags.resetToken, "reset-token", false, "replace the access token and print a new URL")
	remoteAccessCmd.Flags().BoolVar(&remoteFlags.noQR, "no-qr", false, "do not print a QR code")
}

func runRemoteAccess(_ *cobra.Command, _ []string) error {
	inst, err := loadInstance()
	if err != nil {
		return err
	}
	if err := sprites.EnsureInstalled(); err != nil {
		return err
	}
	client := sprites.New()
	if remoteFlags.off {
		return disableRemote(client, inst.SpriteName)
	}
	ensureSync(inst)
	return enableRemote(client, inst.SpriteName, inst.RemoteDir, inst.Branch)
}

func disableRemote(client *sprites.Client, spriteName string) error {
	ui.Step("stopping remote access")
	script := remote.DisableScript()
	if err := remoteRun(client, spriteName, sprites.ExecOptions{Stdin: strings.NewReader(script)}, "bash", "-s"); err != nil {
		return err
	}
	if err := client.SetURLAuth(bg(), spriteName, "sprite"); err != nil {
		ui.Warnf("could not set Sprite URL auth back to private: %v", err)
	}
	ui.Successf("remote access off; Sprite URL is private again")
	ui.Infof("the agent tmux session is unchanged; `portable agent` still attaches")
	return nil
}

func enableRemote(client *sprites.Client, spriteName, remoteDir, branch string) error {
	ui.Step("installing remote access on the workspace")

	uname, err := client.Output(bg(), spriteName, "uname", "-m")
	if err != nil {
		return err
	}
	goarch, err := remote.LinuxGOARCH(uname)
	if err != nil {
		return err
	}
	if err := installLinuxBinary(client, spriteName, goarch); err != nil {
		return err
	}

	token, err := loadOrMakeRemoteToken(client, spriteName)
	if err != nil {
		return err
	}

	sessionCmd, err := remoteSessionCommand()
	if err != nil {
		return err
	}
	script, err := remote.EnableScript(remote.EnableRequest{
		Token:   token,
		Dir:     remoteDir,
		Command: sessionCmd,
	})
	if err != nil {
		return err
	}
	if err := remoteRun(client, spriteName, sprites.ExecOptions{Stdin: strings.NewReader(script)}, "bash", "-s"); err != nil {
		return err
	}

	ui.Step("publishing the workspace URL")
	if err := client.SetURLAuth(bg(), spriteName, "public"); err != nil {
		return fmt.Errorf("make Sprite URL public: %w", err)
	}
	httpInfo, err := client.HTTPURL(bg(), spriteName)
	if err != nil {
		return err
	}
	access, err := remote.AccessURL(httpInfo.URL, token)
	if err != nil {
		return err
	}

	ui.Successf("remote access ready")
	if branch != "" {
		ui.Infof("branch: %s", branch)
	}
	fmt.Println()
	if err := printAccess(os.Stdout, access, remoteFlags.noQR); err != nil {
		return err
	}
	fmt.Println()
	ui.Infof("same session as `portable agent` (close the tab to leave it running)")
	ui.Infof("laptop asleep: edits stay on the workspace until Mutagen resumes")
	ui.Infof("idle Sprite sleeps; opening the URL wakes it (cold wake starts a new tmux session)")
	ui.Infof("stop: portable remote --off")
	return nil
}

// printAccess writes the browser URL. The ?t= value is the access token;
// showing it here is the point of `portable remote`.
func printAccess(w io.Writer, access string, noQR bool) error {
	fmt.Fprintln(w, access)
	if noQR || !term.IsTerminal(int(os.Stdout.Fd())) {
		return nil
	}
	fmt.Fprintln(w)
	return remote.WriteQR(w, access)
}

func remoteSessionCommand() (string, error) {
	if !cfg.SetupDone() {
		return "exec bash -l", nil
	}
	agent, ok := cfg.DefaultAgentConfig()
	if !ok || agent.LocalOnly {
		return "exec bash -l", nil
	}
	run, err := agent.RemoteRun()
	if err != nil {
		return "", err
	}
	return cfg.RemoteShellPreamble() + run, nil
}

func loadOrMakeRemoteToken(client *sprites.Client, spriteName string) (string, error) {
	if !remoteFlags.resetToken {
		out, err := client.Output(bg(), spriteName, "sh", "-c", remote.ReadTokenScript())
		if err != nil {
			return "", err
		}
		if t := parseTokenOutput(out); t != "" {
			return t, nil
		}
	}
	return remote.NewToken()
}

func parseTokenOutput(out string) string {
	for _, line := range strings.Split(out, "\n") {
		t := strings.TrimSpace(line)
		if isHexToken(t) {
			return t
		}
	}
	return ""
}

func isHexToken(s string) bool {
	if len(s) < 32 || len(s)%2 != 0 {
		return false
	}
	for _, c := range s {
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' {
			continue
		}
		return false
	}
	return true
}

func installLinuxBinary(client *sprites.Client, spriteName, goarch string) error {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = projectDir
	}
	src, cleanup, err := remote.LinuxBinary(goarch, Version, remote.FindModuleRoot(cwd))
	if err != nil {
		return err
	}
	defer cleanup()

	ui.Step("copying the web terminal onto the workspace")
	dir, err := os.MkdirTemp("", "portable-push-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	named := filepath.Join(dir, "portable")
	if err := copyFile(src, named, 0o755); err != nil {
		return err
	}
	if err := client.FilePush(bg(), spriteName, named, remote.BinaryPath); err != nil {
		return err
	}
	_, err = client.Output(bg(), spriteName, "chmod", "755", remote.BinaryPath)
	return err
}

func copyFile(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
