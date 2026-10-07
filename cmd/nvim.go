package cmd

import (
	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/sprites"
	"github.com/codegirl-007/portable/internal/ui"
)

var nvimCmd = &cobra.Command{
	Use:                "nvim [file...]",
	Short:              "Open the project in Neovim in the workspace",
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true,
	RunE:               runNvim,
}

func runNvim(_ *cobra.Command, args []string) error {
	inst, err := loadInstance()
	if err != nil {
		return err
	}
	ensureSync(inst)
	ui.Step("starting nvim in workspace")
	// With no file arguments, open the project directory so netrw shows the
	// project files (nvim with no arguments opens an empty buffer).
	if len(args) == 0 {
		args = []string{inst.RemoteDir}
	}
	command := append([]string{"nvim"}, args...)
	return sprites.New().Exec(bg(), inst.SpriteName, sprites.ExecOptions{TTY: true, Dir: inst.RemoteDir}, command...)
}
