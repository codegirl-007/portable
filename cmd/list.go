package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/codegirl-007/portable/internal/mutagen"
	"github.com/codegirl-007/portable/internal/state"
	"github.com/codegirl-007/portable/internal/ui"
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Short:   "List running workspaces",
	Aliases: []string{"ps"},
	RunE: func(_ *cobra.Command, _ []string) error {
		return printWorkspaces(true)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List known workspaces",
	RunE: func(_ *cobra.Command, _ []string) error {
		return printWorkspaces(false)
	},
}

func printWorkspaces(runningOnly bool) error {
	m, err := state.LoadRegistry()
	if err != nil {
		return err
	}
	items := state.Sorted(m)
	if len(items) == 0 {
		if runningOnly {
			ui.Infof("no running workspaces")
		} else {
			ui.Infof("no workspaces registered")
		}
		return nil
	}

	syncStatus, err := mutagen.StatusMap(bg())
	if err != nil {
		return err
	}

	type row struct {
		cols []string
	}
	var rows []row
	for _, in := range items {
		rawStatus := syncStatus[in.SyncSession]
		if runningOnly && !mutagen.SyncRunning(rawStatus) {
			continue
		}
		status := rawStatus
		if status == "" {
			status = "(no session)"
		}
		if runningOnly {
			rows = append(rows, row{[]string{in.SpriteName, status, in.ProjectPath}})
		} else {
			rows = append(rows, row{[]string{in.SpriteName, status, displayRemote(in.RemoteDir), in.ProjectPath}})
		}
	}
	if runningOnly && len(rows) == 0 {
		ui.Infof("no running workspaces")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	if runningOnly {
		fmt.Fprintln(w, "WORKSPACE\tSYNC\tPROJECT")
	} else {
		fmt.Fprintln(w, "WORKSPACE\tSYNC\tREMOTE\tPROJECT")
	}
	for _, r := range rows {
		fmt.Fprintln(w, strings.Join(r.cols, "\t"))
	}
	return w.Flush()
}
