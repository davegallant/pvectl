package cmd

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/spf13/cobra"
)

var ctCmd = &cobra.Command{
	Use:   "ct",
	Short: "Manage containers",
}

var ctListCmd = &cobra.Command{
	Use:   "list",
	Short: "List containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		return runCtList(client)
	},
}

func init() {
	rootCmd.AddCommand(ctCmd)
	ctCmd.AddCommand(ctListCmd)
}

// runCtList fetches and prints a plain VMID/NAME/NODE/STATUS table of
// every container in the cluster.
func runCtList(client *api.Client) error {
	containers, err := client.ListContainers(context.Background())
	if err != nil {
		return fmt.Errorf("listing containers: %w", err)
	}
	fmt.Print(renderContainerList(containers))
	return nil
}

// renderContainerList formats containers as a VMID/NAME/NODE/STATUS
// table from already-fetched data. It performs no I/O, so it's directly
// unit-testable — same pattern as renderNodes/renderStorageReport.
func renderContainerList(containers []api.Container) string {
	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "VMID\tNAME\tNODE\tSTATUS")
	for _, c := range containers {
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", c.VMID, c.Name, c.Node, c.Status)
	}
	_ = tw.Flush()
	return buf.String()
}
