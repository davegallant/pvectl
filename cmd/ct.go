package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/davegallant/pvectl/internal/ssh"
	"github.com/davegallant/pvectl/internal/tui"
	"github.com/spf13/cobra"
)

var ctCmd = &cobra.Command{
	Use:   "ct",
	Short: "A command-line companion for Proxmox",
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

var ctSelectCmd = &cobra.Command{
	Use:               "select [name-or-vmid]",
	Short:             "Pick a container, then pick an action",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeContainerNames,
	RunE:              runCt,
}

func init() {
	rootCmd.AddCommand(ctCmd)
	ctCmd.AddCommand(ctListCmd)
	ctCmd.AddCommand(ctSelectCmd)
}

// runCtList fetches and prints a plain VMID/NAME/NODE/STATUS table of
// every container in the cluster — a static listing, unlike `ct select`'s
// interactive fuzzy-picker.
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

// runCt implements `ct select [name-or-vmid]`: skips the container
// picker when given a name-or-vmid argument (resolveContainer), then
// always shows the action menu — only the container step is optional,
// since choosing the action is select's whole point.
func runCt(cmd *cobra.Command, args []string) error {
	client, err := loadClient()
	if err != nil {
		return friendlySetupError(err)
	}

	c, err := resolveContainer(client, args)
	if err != nil {
		if errors.Is(err, tui.ErrCancelled) {
			return nil
		}
		return err
	}

	action, err := tui.RunActionMenu()
	if err != nil {
		if errors.Is(err, tui.ErrCancelled) {
			return nil
		}
		return err
	}

	announceAction(action, c.Name, c.VMID)
	return dispatchAction(client, action, c)
}

// dispatchAction runs the chosen action menu action against c. Unknown
// actions are a no-op (the menu only ever offers a leaf action from
// tui.ActionTree).
func dispatchAction(client *api.Client, action string, c api.Container) error {
	switch action {
	case "enter":
		return ssh.Enter(c.Node, c.VMID)
	case "start":
		return runStart(client, c)
	case "stop":
		return runStop(client, c)
	case "reboot":
		return runReboot(client, c)
	case "edit":
		return runEdit(client, c.Node, c.VMID)
	case "rename":
		return runRename(client, c)
	case "snapshot":
		return runSnapshot(client, c)
	case "snapshots":
		return runSnapshotsAction(client, c)
	case "delete-snapshot":
		return runDeleteSnapshotAction(client, c)
	case "rollback-snapshot":
		return runRollbackSnapshotAction(client, c)
	case "backup":
		return runBackup(client, c)
	case "backups":
		return runBackups(client, c)
	case "delete-backup":
		return runDeleteBackupAction(client, c)
	case "migrate":
		return runMigrateWithPrompt(client, c)
	case "delete":
		return runDeleteAction(client, c)
	}
	return nil
}
