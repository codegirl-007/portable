package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/config"
	"github.com/codegirl-007/portable/internal/remote"
	"github.com/codegirl-007/portable/internal/sprites"
	"github.com/codegirl-007/portable/internal/ui"
)

var agentCmd = &cobra.Command{
	Use:                "agent [args...]",
	Short:              "Run your configured coding agent in the workspace",
	Long:               "Runs the agent chosen in `portable setup` (OpenCode, Claude Code, etc.). Uses a tmux session on the workspace so you can reattach from `portable remote` or after the laptop disconnects. Detach with Ctrl-b d. Extra args are ignored if a session is already running.",
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true,
	RunE:               runAgent,
}

func runAgent(_ *cobra.Command, args []string) error {
	b, args := takeBranchFlag(args)
	if b != "" {
		flagBranch = b
	}
	if !cfg.SetupDone() {
		return fmt.Errorf("run `portable setup` first (writes %s)", config.GlobalPath())
	}

	agent, ok := cfg.DefaultAgentConfig()
	if !ok {
		return fmt.Errorf("no default agent in config; run `portable setup`")
	}
	if agent.LocalOnly {
		return fmt.Errorf("%s runs on your laptop, not on the workspace\n\n"+
			"Your files sync both ways while the workspace is up. Use Cursor locally and:\n"+
			"  portable run <command>   run tools on the workspace\n"+
			"  portable ssh             remote shell\n\n"+
			"To run a remote CLI agent instead, run `portable setup` and pick OpenCode or Claude Code",
			agent.Label)
	}
	run, err := agent.RemoteRun()
	if err != nil {
		return err
	}

	inst, err := loadInstance()
	if err != nil {
		return err
	}
	ensureSync(inst)
	ui.Step("starting %s in workspace", agent.Label)

	command := cfg.RemoteShellPreamble() + run
	if extraAgentArgsIgnored(tmuxSessionExists(inst.SpriteName, remote.SessionName), len(args)) {
		ui.Warnf("agent session already running; extra arguments ignored (detach with Ctrl-b d)")
	} else if len(args) > 0 {
		quoted := make([]string, len(args))
		for i, a := range args {
			quoted[i] = shellQuote(a)
		}
		command += " " + strings.Join(quoted, " ")
	}
	// Attach to the same tmux session the phone/browser uses, so the agent
	// keeps running if this laptop disconnects.
	wrapped := remote.TmuxAttachOrCreate(remote.SessionName, inst.RemoteDir, command)
	return sprites.New().Exec(bg(), inst.SpriteName, sprites.ExecOptions{TTY: true, Dir: inst.RemoteDir}, "sh", "-c", wrapped)
}

func extraAgentArgsIgnored(sessionExists bool, nArgs int) bool {
	return sessionExists && nArgs > 0
}

func tmuxSessionExists(sprite, session string) bool {
	_, err := sprites.New().Output(bg(), sprite, "tmux", "has-session", "-t", session)
	return err == nil
}
