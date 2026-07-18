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

var qmCmd = &cobra.Command{
	Use:   "qm",
	Short: "Fuzzy-find and manage QEMU VMs",
}

var qmListCmd = &cobra.Command{
	Use:   "list",
	Short: "List VMs",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		return runQmList(client)
	},
}

var qmSelectCmd = &cobra.Command{
	Use:   "select [name-or-vmid]",
	Short: "Pick a VM, then pick an action",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runQm,
}

func init() {
	rootCmd.AddCommand(qmCmd)
	qmCmd.AddCommand(qmListCmd)
	qmCmd.AddCommand(qmSelectCmd)
}

// runQmList fetches and prints a plain VMID/NAME/NODE/STATUS table of
// every VM in the cluster — a static listing, unlike `qm select`'s
// interactive fuzzy-picker.
func runQmList(client *api.Client) error {
	vms, err := client.ListVMs(context.Background())
	if err != nil {
		return fmt.Errorf("listing VMs: %w", err)
	}
	fmt.Print(renderVMList(vms))
	return nil
}

// renderVMList formats vms as a VMID/NAME/NODE/STATUS table from
// already-fetched data. It performs no I/O, so it's directly
// unit-testable — same pattern as renderContainerList.
func renderVMList(vms []api.VM) string {
	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "VMID\tNAME\tNODE\tSTATUS")
	for _, v := range vms {
		_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", v.VMID, v.Name, v.Node, v.Status)
	}
	_ = tw.Flush()
	return buf.String()
}

// runQm is runCt's mirror for QEMU VMs.
func runQm(cmd *cobra.Command, args []string) error {
	client, err := loadClient()
	if err != nil {
		return friendlySetupError(err)
	}

	v, err := resolveVM(client, args)
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

	announceAction(action, v.Name, v.VMID)
	return dispatchVMAction(client, action, v)
}

// dispatchVMAction runs the chosen action menu action against v. Unknown
// actions are a no-op (the menu only ever offers a leaf action from
// tui.ActionTree).
func dispatchVMAction(client *api.Client, action string, v api.VM) error {
	switch action {
	case "enter":
		return ssh.EnterVM(v.Node, v.VMID)
	case "start":
		return runStartVM(client, v)
	case "stop":
		return runStopVM(client, v)
	case "reboot":
		return runRebootVM(client, v)
	case "edit":
		return runEditVM(client, v.Node, v.VMID)
	case "rename":
		return runRenameVM(client, v)
	case "snapshot":
		return runSnapshotVM(client, v)
	case "snapshots":
		return runSnapshotsVMAction(client, v)
	case "delete-snapshot":
		return runDeleteSnapshotVMAction(client, v)
	case "rollback-snapshot":
		return runRollbackSnapshotVMAction(client, v)
	case "backup":
		return runBackupVM(client, v)
	case "backups":
		return runBackupsVM(client, v)
	case "delete-backup":
		return runDeleteBackupVMAction(client, v)
	case "migrate":
		return runMigrateVMWithPrompt(client, v)
	case "delete":
		return runDeleteVMAction(client, v)
	}
	return nil
}
