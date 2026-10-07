package cmd

import (
	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/sprites"
	"github.com/codegirl-007/portable/internal/ui"
)

var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "Open a shell in the workspace",
	RunE:  runSSH,
}

func runSSH(_ *cobra.Command, _ []string) error {
	inst, err := loadInstance()
	if err != nil {
		return err
	}
	ensureSync(inst)
	ui.Step("connecting to workspace")
	return sprites.New().Exec(bg(), inst.SpriteName, sprites.ExecOptions{TTY: true, Dir: inst.RemoteDir}, "bash", "-l")
}
