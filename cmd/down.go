package cmd

import (
	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/mutagen"
	"github.com/codegirl-007/portable/internal/ui"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Pause sync so the workspace can sleep (free)",
	RunE:  runDown,
}

func runDown(_ *cobra.Command, _ []string) error {
	inst, err := loadInstance()
	if err != nil {
		return err
	}
	ui.Step("pausing sync...")
	if err := mutagen.Pause(bg(), inst.SyncSession); err != nil {
		ui.Warnf("%v", err)
	}
	ui.Successf("asleep; `portable up` to resume, `portable destroy` to delete")
	return nil
}
