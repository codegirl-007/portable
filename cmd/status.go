package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/mutagen"
	"github.com/codegirl-007/portable/internal/state"
	"github.com/codegirl-007/portable/internal/ui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current project's workspace",
	RunE:  runStatus,
}

func runStatus(_ *cobra.Command, _ []string) error {
	inst, err := state.Load(projectDir)
	if err != nil {
		return err
	}
	if inst == nil || inst.SpriteName == "" {
		ui.Infof("no workspace for %s", projectDir)
		return nil
	}
	fmt.Printf("project : %s\n", inst.ProjectPath)
	fmt.Printf("workspace: %s\n", inst.SpriteName)
	fmt.Printf("local   : %s\n", inst.LocalDir)
	fmt.Printf("remote  : %s\n", displayRemote(inst.RemoteDir))
	if sessions, err := mutagen.List(bg()); err == nil {
		for _, s := range sessions {
			if s.Name == inst.SyncSession {
				fmt.Printf("sync    : %s\n", s.Status)
			}
		}
	}
	return nil
}
