package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/sprites"
)

var runCmd = &cobra.Command{
	Use:                "run <command> [args...]",
	Short:              "Run a command in the workspace",
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true,
	RunE:               runRun,
}

func runRun(_ *cobra.Command, args []string) error {
	b, args := takeBranchFlag(args)
	if b != "" {
		flagBranch = b
	}
	if len(args) == 0 {
		return fmt.Errorf("run requires a command")
	}
	inst, err := loadInstance()
	if err != nil {
		return err
	}
	ensureSync(inst)
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}
	command := cfg.RemoteShellPreamble() + strings.Join(quoted, " ")
	return sprites.New().Exec(bg(), inst.SpriteName, sprites.ExecOptions{Dir: inst.RemoteDir}, "sh", "-c", command)
}
