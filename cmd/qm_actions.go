package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/spf13/cobra"
)

func runStartVM(client *api.Client, v api.VM) error {
	upid, err := client.StartVM(context.Background(), v.Node, v.VMID)
	if err != nil {
		return fmt.Errorf("starting %s (%d): %w", v.Name, v.VMID, err)
	}
	return runProgressAction(client, v.Node, upid,
		fmt.Sprintf("starting %s (%d)", v.Name, v.VMID),
		fmt.Sprintf("started %s (%d)", v.Name, v.VMID))
}

func runStopVM(client *api.Client, v api.VM) error {
	upid, err := client.StopVM(context.Background(), v.Node, v.VMID)
	if err != nil {
		return fmt.Errorf("stopping %s (%d): %w", v.Name, v.VMID, err)
	}
	return runProgressAction(client, v.Node, upid,
		fmt.Sprintf("stopping %s (%d)", v.Name, v.VMID),
		fmt.Sprintf("stopped %s (%d)", v.Name, v.VMID))
}

func runRebootVM(client *api.Client, v api.VM) error {
	upid, err := client.RebootVM(context.Background(), v.Node, v.VMID)
	if err != nil {
		return fmt.Errorf("rebooting %s (%d): %w", v.Name, v.VMID, err)
	}
	return runProgressAction(client, v.Node, upid,
		fmt.Sprintf("rebooting %s (%d)", v.Name, v.VMID),
		fmt.Sprintf("rebooted %s (%d)", v.Name, v.VMID))
}

// qmSnapshotName is ctSnapshotName's mirror for QEMU VMs — see its comment.
var qmSnapshotName string

func runSnapshotVM(client *api.Client, v api.VM) error {
	name := qmSnapshotName
	if name == "" {
		fmt.Print("snapshot-name: ")
		reader := bufio.NewReader(os.Stdin)
		nameLine, _ := reader.ReadString('\n')
		name = strings.TrimSpace(nameLine)
	}
	if name == "" {
		return fmt.Errorf("snapshot name required")
	}

	upid, err := client.SnapshotVM(context.Background(), v.Node, v.VMID, name)
	if err != nil {
		return fmt.Errorf("snapshotting %s (%d): %w", v.Name, v.VMID, err)
	}
	return runProgressAction(client, v.Node, upid,
		fmt.Sprintf("snapshotting %s (%d)", v.Name, v.VMID),
		fmt.Sprintf("snapshotted %s (%d)", v.Name, v.VMID))
}

// qmBackupStorage is ctBackupStorage's mirror for QEMU VMs — see its comment.
var qmBackupStorage string

func runBackupVM(client *api.Client, v api.VM) error {
	storage := qmBackupStorage
	if storage == "" {
		storage = promptStorage(client, v.Node)
	}

	upid, err := client.Backup(context.Background(), v.Node, v.VMID, storage)
	if err != nil {
		return fmt.Errorf("backing up %s (%d): %w", v.Name, v.VMID, err)
	}
	return runProgressAction(client, v.Node, upid,
		fmt.Sprintf("backing up %s (%d)", v.Name, v.VMID),
		fmt.Sprintf("backed up %s (%d)", v.Name, v.VMID))
}

// runMigrateVMWithPrompt prompts for a target node interactively, then
// migrates v — used by `qm migrate <name-or-vmid>` when no `--target` is
// given. The direct `qm migrate <name-or-vmid> --target <node>` form
// (qmMigrateCmd in migrate.go) resolves the target some other way and
// calls runMigrateVM directly instead, so it never touches stdin.
func runMigrateVMWithPrompt(client *api.Client, v api.VM) error {
	target, err := promptTargetNode(client, v.Node)
	if err != nil {
		return err
	}
	return runMigrateVM(client, v, target)
}

func runMigrateVM(client *api.Client, v api.VM, target string) error {
	online := v.Status == "running"
	label := fmt.Sprintf("migrating %s (%d) to %s", v.Name, v.VMID, target)
	if online {
		label += " (live)"
	}

	upid, err := client.MigrateVM(context.Background(), v.Node, v.VMID, target, online)
	if err != nil {
		return fmt.Errorf("migrating %s (%d): %w", v.Name, v.VMID, err)
	}
	progressErr := runProgressAction(client, v.Node, upid, label,
		fmt.Sprintf("migrated %s (%d) to %s", v.Name, v.VMID, target))
	printTaskLogIfVerbose(client, v.Node, upid)
	return progressErr
}

func runBackupsVM(client *api.Client, v api.VM) error {
	return runListBackups(client, v.Node, v.VMID, v.Name)
}

// qmBackupsDeleteVolID/qmBackupsDeleteYes and qmSnapshotsDeleteName/
// qmSnapshotsDeleteYes are runDeleteBackupAction's /
// runDeleteSnapshotAction's mirror for QEMU VMs — see their comment.
var qmBackupsDeleteVolID string
var qmBackupsDeleteYes bool
var qmSnapshotsDeleteName string
var qmSnapshotsDeleteYes bool
var qmSnapshotsRollbackName string
var qmSnapshotsRollbackYes bool

func runDeleteBackupVMAction(client *api.Client, v api.VM) error {
	return runDeleteBackup(client, v.Node, v.VMID, v.Name, qmBackupsDeleteVolID, qmBackupsDeleteYes)
}

// qmRestoreVolID/qmRestoreStorage/qmRestoreYes are ctRestoreVolID's /
// ctRestoreStorage's / ctRestoreYes's mirror for QEMU VMs — see their
// comment.
var qmRestoreVolID string
var qmRestoreStorage string
var qmRestoreYes bool

// promptImagesStorage lists node's storages that support QEMU disk
// images ("images" content) — restore's mirror of promptRootfsStorage
// (ct_create.go), which filters on "rootdir" for LXC root filesystems
// instead.
func promptImagesStorage(client *api.Client, node string) (string, error) {
	storages, err := client.ListNodeStorages(context.Background(), node)
	if err != nil {
		return "", fmt.Errorf("listing storages on %s: %w", node, err)
	}

	var names []string
	for _, s := range storages {
		if s.SupportsContent("images") {
			names = append(names, s.Storage)
		}
	}
	sort.Strings(names)
	return promptChoice("storage", names)
}

// runRestoreBackupVMAction is runRestoreBackupAction's mirror for QEMU
// VMs, using promptImagesStorage ("images" content) in place of
// promptRootfsStorage ("rootdir").
func runRestoreBackupVMAction(client *api.Client, v api.VM) error {
	storage := qmRestoreStorage
	if storage == "" {
		var err error
		storage, err = promptImagesStorage(client, v.Node)
		if err != nil {
			return err
		}
	}
	return runRestoreBackup(client, v.Node, v.VMID, v.Name, "VM", qmRestoreVolID, storage, qmRestoreYes, client.RestoreVM)
}

// qmRestoreNode/qmRestoreVMID are ctRestoreNode's/ctRestoreVMID's mirror
// for QEMU VMs — see their comment.
var qmRestoreNode string
var qmRestoreVMID int

// runQmBackupsRestore is runCtBackupsRestore's mirror for QEMU VMs.
func runQmBackupsRestore(cmd *cobra.Command, args []string) error {
	nodeMode := cmd.Flags().Changed("node")
	if nodeMode && len(args) > 0 {
		return fmt.Errorf("cannot combine a name-or-vmid argument with --node")
	}

	client, err := loadClient()
	if err != nil {
		return friendlySetupError(err)
	}

	if nodeMode {
		return runRestoreFromNodeQM(client, qmRestoreNode)
	}

	v, err := resolveVM(client, args)
	if err != nil {
		return err
	}
	return runRestoreBackupVMAction(client, v)
}

// runRestoreFromNodeQM is runRestoreFromNodeCT's mirror for QEMU VMs.
func runRestoreFromNodeQM(client *api.Client, node string) error {
	backups, err := fetchAllBackups(client, node)
	if err != nil {
		return fmt.Errorf("listing backups on %s: %w", node, err)
	}
	backups = filterBackupsByGuestType(backups, "qemu")

	storage := qmRestoreStorage
	if storage == "" {
		var err error
		storage, err = promptImagesStorage(client, node)
		if err != nil {
			return err
		}
	}

	return runRestoreFromNode(client, node, "VM", backups, qmRestoreVMID, qmRestoreVolID, storage, qmRestoreYes, client.RestoreVM,
		func(vmid int) (bool, error) { return vmExists(client, vmid) })
}

func runSnapshotsVMAction(client *api.Client, v api.VM) error {
	return runListSnapshotsVM(client, v.Node, v.VMID, v.Name)
}

func runDeleteSnapshotVMAction(client *api.Client, v api.VM) error {
	return runDeleteSnapshotVM(client, v.Node, v.VMID, v.Name, qmSnapshotsDeleteName, qmSnapshotsDeleteYes)
}

func runRollbackSnapshotVMAction(client *api.Client, v api.VM) error {
	return runRollbackSnapshotVM(client, v.Node, v.VMID, v.Name, qmSnapshotsRollbackName, qmSnapshotsRollbackYes)
}

// newSimpleVMActionCmd is newSimpleActionCmd's mirror for QEMU VMs.
func newSimpleVMActionCmd(use, short string, run func(*api.Client, api.VM) error) *cobra.Command {
	return &cobra.Command{
		Use:               use + " <name-or-vmid>",
		Short:             short,
		Args:              requireArgs("name-or-vmid"),
		ValidArgsFunction: completeVMNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := loadClient()
			if err != nil {
				return friendlySetupError(err)
			}
			v, err := resolveVM(client, args)
			if err != nil {
				return err
			}
			return run(client, v)
		},
	}
}

func init() {
	qmCmd.AddCommand(newSimpleVMActionCmd("start", "Start a VM", runStartVM))
	qmCmd.AddCommand(newSimpleVMActionCmd("stop", "Stop a VM", runStopVM))
	qmCmd.AddCommand(newSimpleVMActionCmd("reboot", "Reboot a VM", runRebootVM))

	// Mirrors actions.go's ct registration — no top-level `qm backup`/
	// `qm snapshot`; creation nests under the plural group command like
	// every other verb, for consistency.
	backupsCmd := &cobra.Command{
		Use:   "backups",
		Short: "Manage a VM's backups",
	}
	qmBackupCreateCmd := newSimpleVMActionCmd("create", "Create a backup", runBackupVM)
	qmBackupCreateCmd.Flags().StringVar(&qmBackupStorage, "storage", "", "backup storage target (skips the interactive prompt when set, along with the name-or-vmid argument)")
	backupsCmd.AddCommand(qmBackupCreateCmd)
	backupsCmd.AddCommand(newSimpleVMActionCmd("list", "List a VM's backups", runBackupsVM))
	qmBackupsDeleteCmd := newSimpleVMActionCmd("delete", "Delete one of a VM's backups", runDeleteBackupVMAction)
	qmBackupsDeleteCmd.Flags().StringVar(&qmBackupsDeleteVolID, "volid", "", "backup volid to delete (skips the interactive listing/prompt when set, along with the name-or-vmid argument)")
	qmBackupsDeleteCmd.Flags().BoolVarP(&qmBackupsDeleteYes, "yes", "y", false, "skip the confirmation prompt")
	backupsCmd.AddCommand(qmBackupsDeleteCmd)

	qmBackupRestoreCmd := &cobra.Command{
		Use:               "restore [name-or-vmid]",
		Short:             "Restore a VM from a backup",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeVMNames,
		RunE:              runQmBackupsRestore,
	}
	qmBackupRestoreCmd.Flags().StringVar(&qmRestoreVolID, "volid", "", "backup volid to restore (skips the interactive listing/prompt when set)")
	qmBackupRestoreCmd.Flags().StringVar(&qmRestoreStorage, "storage", "", "target storage for the restored VM (skips the interactive prompt when set)")
	qmBackupRestoreCmd.Flags().BoolVarP(&qmRestoreYes, "yes", "y", false, "skip the confirmation prompt")
	qmBackupRestoreCmd.Flags().StringVar(&qmRestoreNode, "node", "", "restore from any backup on this node instead of an existing VM's own backups (disaster recovery — cannot be combined with a name-or-vmid argument)")
	qmBackupRestoreCmd.Flags().IntVar(&qmRestoreVMID, "vmid", 0, "target vmid for --node mode (default: the vmid recorded in the chosen backup)")
	backupsCmd.AddCommand(qmBackupRestoreCmd)

	qmCmd.AddCommand(backupsCmd)

	snapshotsCmd := &cobra.Command{
		Use:   "snapshots",
		Short: "Manage a VM's snapshots",
	}
	qmSnapshotCreateCmd := newSimpleVMActionCmd("create", "Create a snapshot", runSnapshotVM)
	qmSnapshotCreateCmd.Flags().StringVar(&qmSnapshotName, "snapshot-name", "", "snapshot name (skips the interactive prompt when set, along with the name-or-vmid argument)")
	snapshotsCmd.AddCommand(qmSnapshotCreateCmd)
	snapshotsCmd.AddCommand(newSimpleVMActionCmd("list", "List a VM's snapshots", runSnapshotsVMAction))
	qmSnapshotsDeleteCmd := newSimpleVMActionCmd("delete", "Delete one of a VM's snapshots", runDeleteSnapshotVMAction)
	qmSnapshotsDeleteCmd.Flags().StringVar(&qmSnapshotsDeleteName, "snapshot-name", "", "snapshot name to delete (skips the interactive listing/prompt when set, along with the name-or-vmid argument)")
	qmSnapshotsDeleteCmd.Flags().BoolVarP(&qmSnapshotsDeleteYes, "yes", "y", false, "skip the confirmation prompt")
	snapshotsCmd.AddCommand(qmSnapshotsDeleteCmd)
	qmSnapshotsRollbackCmd := newSimpleVMActionCmd("rollback", "Roll back a VM to one of its snapshots", runRollbackSnapshotVMAction)
	qmSnapshotsRollbackCmd.Flags().StringVar(&qmSnapshotsRollbackName, "snapshot-name", "", "snapshot name to roll back to (skips the interactive listing/prompt when set, along with the name-or-vmid argument)")
	qmSnapshotsRollbackCmd.Flags().BoolVarP(&qmSnapshotsRollbackYes, "yes", "y", false, "skip the confirmation prompt")
	snapshotsCmd.AddCommand(qmSnapshotsRollbackCmd)
	qmCmd.AddCommand(snapshotsCmd)
}
