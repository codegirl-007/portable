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
	Use:   "destroy",
	Short: "Permanently delete the project's workspace",
	RunE:  runDestroy,
}

func runDestroy(_ *cobra.Command, _ []string) error {
	inst, err := loadInstance()
	if err != nil {
		return err
	}

	_ = mutagen.Terminate(bg(), inst.SyncSession)

	ui.Step("destroying the workspace")
	if err := sprites.New().Destroy(bg(), inst.SpriteName); err != nil {
		return err
	}
	_ = sshconfig.Remove(inst.SpriteName)
	_ = state.Remove(projectDir)
	_ = state.Unregister(projectDir)

	ui.Successf("destroyed the workspace")
	return nil
}
