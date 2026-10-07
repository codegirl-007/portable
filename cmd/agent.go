package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/config"
	"github.com/codegirl-007/portable/internal/sprites"
	"github.com/codegirl-007/portable/internal/ui"
)

var agentCmd = &cobra.Command{
	Use:                "agent [args...]",
	Short:              "Run your configured coding agent in the workspace",
	Long:               "Runs the agent chosen in `portable setup` (OpenCode, Claude Code, etc.).",
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true,
	RunE:               runAgent,
}

func runAgent(_ *cobra.Command, args []string) error {
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
	if strings.TrimSpace(agent.Run) == "" {
		return fmt.Errorf("agent %q has no run command; run `portable setup`", agent.Name)
	}

	inst, err := loadInstance()
	if err != nil {
		return err
	}
	ensureSync(inst)
	ui.Step("starting %s in workspace", agent.Label)

	command := cfg.RemoteShellPreamble() + agent.Run
	if len(args) > 0 {
		quoted := make([]string, len(args))
		for i, a := range args {
			quoted[i] = shellQuote(a)
		}
		command += " " + strings.Join(quoted, " ")
	}
	return sprites.New().Exec(bg(), inst.SpriteName, sprites.ExecOptions{TTY: true, Dir: inst.RemoteDir}, "sh", "-c", command)
}
