package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/config"
	"github.com/codegirl-007/portable/internal/setup"
	"github.com/codegirl-007/portable/internal/ui"
)

var setupFlags struct {
	force bool
	agent string
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure portable once for all projects",
	Long:  "Interactive wizard that writes ~/.config/portable/config.toml (your coding agent, dotfiles, and workspace extras).",
	RunE:  runSetup,
}

func init() {
	setupCmd.Flags().BoolVar(&setupFlags.force, "force", false, "overwrite existing global config without prompting")
	setupCmd.Flags().StringVar(&setupFlags.agent, "agent", "", "non-interactive: opencode, claude, codex, or cursor")
}

func runSetup(_ *cobra.Command, _ []string) error {
	var result *setup.WizardResult
	var err error

	switch {
	case setupFlags.agent != "":
		if !validAgentID(setupFlags.agent) {
			return fmt.Errorf("unknown agent %q (use opencode, claude, codex, or cursor)", setupFlags.agent)
		}
		result = &setup.WizardResult{AgentID: setupFlags.agent, Gh: false, Nvim: false}
	case ui.IsTerminal():
		result, err = setup.RunInteractive(setupFlags.force)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("run from a terminal or pass --agent opencode|claude|codex|cursor")
	}

	cfg := setup.BuildConfig(result.AgentID, result.Nvim, result.Gh, true, result.Dotfiles, result.Tools)
	if err := config.WriteGlobal(cfg); err != nil {
		return err
	}

	agent, _ := cfg.DefaultAgentConfig()
	if agent != nil {
		if _, _, warn := setup.ResolveCredentials(*agent); warn != "" {
			ui.Warnf("%s", warn)
		}
	}

	ui.Successf("wrote %s", config.GlobalPath())
	fmt.Println()
	ui.Infof("Next: cd into a project and run `portable up`")
	if agent != nil {
		ui.Infof("Agent: `portable agent` starts %s on the workspace", agent.Label)
	}
	ui.Infof("Changed agents later? Run `portable setup` again, then `portable up --reprovision`")
	return nil
}

func validAgentID(id string) bool {
	for _, c := range setup.Choices() {
		if c.ID == id {
			return true
		}
	}
	return false
}
