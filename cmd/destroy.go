package cmd

import (
	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/mutagen"
	"github.com/codegirl-007/portable/internal/sprites"
	"github.com/codegirl-007/portable/internal/sshconfig"
	"github.com/codegirl-007/portable/internal/state"
	"github.com/codegirl-007/portable/internal/ui"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy [workspace]",
	Short: "Permanently delete a workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDestroy,
}

func runDestroy(_ *cobra.Command, args []string) error {
	inst, err := resolveInstance(args)
	if err != nil {
		return err
	}

	_ = mutagen.Terminate(bg(), inst.SyncSession)

	ui.Step("destroying the workspace")
	if err := sprites.New().Destroy(bg(), inst.SpriteName); err != nil {
		return err
	}
	_ = sshconfig.Remove(inst.SpriteName)
	_ = state.Remove(inst.ProjectPath)
	_ = state.Unregister(inst.ProjectPath)

	ui.Successf("destroyed the workspace")
	return nil
}
