package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/sprites"
	"github.com/codegirl-007/portable/internal/ui"
)

var opencodeCmd = &cobra.Command{
	Use:                "opencode [args...]",
	Short:              "Run opencode in the workspace",
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: true,
	RunE:               runOpencode,
}

func runOpencode(_ *cobra.Command, args []string) error {
	inst, err := loadInstance()
	if err != nil {
		return err
	}
	ensureSync(inst)
	ui.Step("starting opencode in workspace")

	command := remoteOpencodeEnv + "exec opencode"
	if len(args) > 0 {
		quoted := make([]string, len(args))
		for i, a := range args {
			quoted[i] = shellQuote(a)
		}
		command += " " + strings.Join(quoted, " ")
	}
	return sprites.New().Exec(bg(), inst.SpriteName, sprites.ExecOptions{TTY: true, Dir: inst.RemoteDir}, "sh", "-c", command)
}
