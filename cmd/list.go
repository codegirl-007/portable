package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/state"
	"github.com/codegirl-007/portable/internal/ui"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List known workspaces",
	RunE:  runList,
}

func runList(_ *cobra.Command, _ []string) error {
	m, err := state.LoadRegistry()
	if err != nil {
		return err
	}
	items := state.Sorted(m)
	if len(items) == 0 {
		ui.Infof("no workspaces registered")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "WORKSPACE\tREMOTE\tPROJECT")
	for _, in := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\n", in.SpriteName, displayRemote(in.RemoteDir), in.ProjectPath)
	}
	return w.Flush()
}
