package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/spf13/cobra"
)

func runStart(client *api.Client, c api.Container) error {
	upid, err := client.Start(context.Background(), c.Node, c.VMID)
	if err != nil {
		return fmt.Errorf("starting %s (%d): %w", c.Name, c.VMID, err)
	}
	return runProgressAction(client, c.Node, upid,
		fmt.Sprintf("starting %s (%d)", c.Name, c.VMID),
		fmt.Sprintf("started %s (%d)", c.Name, c.VMID))
}

func runStop(client *api.Client, c api.Container) error {
	upid, err := client.Stop(context.Background(), c.Node, c.VMID)
	if err != nil {
		return fmt.Errorf("stopping %s (%d): %w", c.Name, c.VMID, err)
	}
	return runProgressAction(client, c.Node, upid,
		fmt.Sprintf("stopping %s (%d)", c.Name, c.VMID),
		fmt.Sprintf("stopped %s (%d)", c.Name, c.VMID))
}

func runReboot(client *api.Client, c api.Container) error {
	upid, err := client.Reboot(context.Background(), c.Node, c.VMID)
	if err != nil {
		return fmt.Errorf("rebooting %s (%d): %w", c.Name, c.VMID, err)
	}
	return runProgressAction(client, c.Node, upid,
		fmt.Sprintf("rebooting %s (%d)", c.Name, c.VMID),
		fmt.Sprintf("rebooted %s (%d)", c.Name, c.VMID))
}

// ctSnapshotName backs `ct snapshots create`'s `--name` flag, which skips
// the interactive name prompt.
var ctSnapshotName string

func runSnapshot(client *api.Client, c api.Container) error {
	name := ctSnapshotName
	if name == "" {
		fmt.Print("snapshot-name: ")
		reader := bufio.NewReader(os.Stdin)
		nameLine, _ := reader.ReadString('\n')
		name = strings.TrimSpace(nameLine)
	}
	if name == "" {
		return fmt.Errorf("snapshot name required")
	}

	upid, err := client.Snapshot(context.Background(), c.Node, c.VMID, name)
	if err != nil {
		return fmt.Errorf("snapshotting %s (%d): %w", c.Name, c.VMID, err)
	}
	return runProgressAction(client, c.Node, upid,
		fmt.Sprintf("snapshotting %s (%d)", c.Name, c.VMID),
		fmt.Sprintf("snapshotted %s (%d)", c.Name, c.VMID))
}

// ctBackupStorage backs `ct backups create`'s `--storage` flag, which
// skips the interactive storage prompt.
var ctBackupStorage string

func runBackup(client *api.Client, c api.Container) error {
	storage := ctBackupStorage
	if storage == "" {
		storage = promptStorage(client, c.Node)
	}

	upid, err := client.Backup(context.Background(), c.Node, c.VMID, storage)
	if err != nil {
		return fmt.Errorf("backing up %s (%d): %w", c.Name, c.VMID, err)
	}
	return runProgressAction(client, c.Node, upid,
		fmt.Sprintf("backing up %s (%d)", c.Name, c.VMID),
		fmt.Sprintf("backed up %s (%d)", c.Name, c.VMID))
}

// runMigrateWithPrompt prompts for a target node interactively, then
// migrates c — used by `ct migrate <name-or-vmid>` when no `--target` is
// given. The direct `ct migrate <name-or-vmid> --target <node>` form
// (ctMigrateCmd in migrate.go) resolves the target some other way and
// calls runMigrate directly instead, so it never touches stdin.
func runMigrateWithPrompt(client *api.Client, c api.Container) error {
	printRestartNotice(c)
	target, err := promptTargetNode(client, c.Node)
	if err != nil {
		return err
	}
	return runMigrate(client, c, target)
}

func runMigrate(client *api.Client, c api.Container, target string) error {
	restart := c.Status == "running"
	label := fmt.Sprintf("migrating %s (%d) to %s", c.Name, c.VMID, target)

	upid, err := client.Migrate(context.Background(), c.Node, c.VMID, target, restart)
	if err != nil {
		return fmt.Errorf("migrating %s (%d): %w", c.Name, c.VMID, err)
	}
	progressErr := runProgressAction(client, c.Node, upid, label,
		fmt.Sprintf("migrated %s (%d) to %s", c.Name, c.VMID, target))
	printTaskLogIfVerbose(client, c.Node, upid)
	return progressErr
}

func runBackups(client *api.Client, c api.Container) error {
	return runListBackups(client, c.Node, c.VMID, c.Name)
}

// ctBackupsDeleteVolID/ctBackupsDeleteYes and ctSnapshotsDeleteName/
// ctSnapshotsDeleteYes back the `--volid`/`--name` and `-y`/`--yes` flags
// on `ct backups delete`/`ct snapshots delete`. Given together they skip
// the interactive listing/prompt and the "type yes" confirmation
// entirely, making deletion scriptable.
var ctBackupsDeleteVolID string
var ctBackupsDeleteYes bool
var ctSnapshotsDeleteName string
var ctSnapshotsDeleteYes bool
var ctSnapshotsRollbackName string
var ctSnapshotsRollbackYes bool

func runDeleteBackupAction(client *api.Client, c api.Container) error {
	return runDeleteBackup(client, c.Node, c.VMID, c.Name, ctBackupsDeleteVolID, ctBackupsDeleteYes)
}

// ctRestoreVolID/ctRestoreStorage/ctRestoreYes back `ct backups
// restore`'s `--volid`/`--storage`/`-y`/`--yes` flags, same convention as
// the backup/snapshot delete flags above.
var ctRestoreVolID string
var ctRestoreStorage string
var ctRestoreYes bool

// runRestoreBackupAction restores c in place from one of its own
// backups, resolving the target storage from ctRestoreStorage or an
// interactive prompt (content-filtered to "rootdir", like ct create's
// own storage prompt) first.
func runRestoreBackupAction(client *api.Client, c api.Container) error {
	storage := ctRestoreStorage
	if storage == "" {
		var err error
		storage, err = promptRootfsStorage(client, c.Node)
		if err != nil {
			return err
		}
	}
	return runRestoreBackup(client, c.Node, c.VMID, c.Name, "container", ctRestoreVolID, storage, ctRestoreYes, client.RestoreContainer)
}

// ctRestoreNode/ctRestoreVMID back `ct backups restore`'s `--node`/
// `--vmid` flags. --node's presence (not merely being non-empty — see
// cmd.Flags().Changed in runCtBackupsRestore) is the switch into
// disaster-recovery mode: restoring from any backup found on that node
// rather than an existing guest's own backups, for a guest that no
// longer exists to be resolved via resolveContainer.
var ctRestoreNode string
var ctRestoreVMID int

// runCtBackupsRestore is `ct backups restore [name-or-vmid]`'s RunE.
// Unlike every other simple action it can't use newSimpleActionCmd,
// since disaster-recovery mode deliberately has no existing guest to
// resolve — --node's presence branches before resolveContainer is ever
// called.
func runCtBackupsRestore(cmd *cobra.Command, args []string) error {
	nodeMode := cmd.Flags().Changed("node")
	if nodeMode && len(args) > 0 {
		return fmt.Errorf("cannot combine a name-or-vmid argument with --node")
	}

	client, err := loadClient()
	if err != nil {
		return friendlySetupError(err)
	}

	if nodeMode {
		return runRestoreFromNodeCT(client, ctRestoreNode)
	}

	c, err := resolveContainer(client, args)
	if err != nil {
		return err
	}
	return runRestoreBackupAction(client, c)
}

// runRestoreFromNodeCT is runCtBackupsRestore's disaster-recovery branch:
// lists every backup on node, filtered to LXC archives
// (filterBackupsByGuestType), and restores the chosen one via
// runRestoreFromNode.
func runRestoreFromNodeCT(client *api.Client, node string) error {
	backups, err := fetchAllBackups(client, node)
	if err != nil {
		return fmt.Errorf("listing backups on %s: %w", node, err)
	}
	backups = filterBackupsByGuestType(backups, "lxc")

	storage := ctRestoreStorage
	if storage == "" {
		var err error
		storage, err = promptRootfsStorage(client, node)
		if err != nil {
			return err
		}
	}

	return runRestoreFromNode(client, node, "container", backups, ctRestoreVMID, ctRestoreVolID, storage, ctRestoreYes, client.RestoreContainer,
		func(vmid int) (bool, error) { return containerExists(client, vmid) })
}

func runSnapshotsAction(client *api.Client, c api.Container) error {
	return runListSnapshots(client, c.Node, c.VMID, c.Name)
}

func runDeleteSnapshotAction(client *api.Client, c api.Container) error {
	return runDeleteSnapshot(client, c.Node, c.VMID, c.Name, ctSnapshotsDeleteName, ctSnapshotsDeleteYes)
}

func runRollbackSnapshotAction(client *api.Client, c api.Container) error {
	return runRollbackSnapshot(client, c.Node, c.VMID, c.Name, ctSnapshotsRollbackName, ctSnapshotsRollbackYes)
}

// newSimpleActionCmd builds a `ct <use> <name-or-vmid>` command: it acts
// directly on the named/vmid'd container (via resolveContainer/
// findContainer).
func newSimpleActionCmd(use, short string, run func(*api.Client, api.Container) error) *cobra.Command {
	return &cobra.Command{
		Use:               use + " <name-or-vmid>",
		Short:             short,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeContainerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := loadClient()
			if err != nil {
				return friendlySetupError(err)
			}
			c, err := resolveContainer(client, args)
			if err != nil {
				return err
			}
			return run(client, c)
		},
	}
}

func init() {
	ctCmd.AddCommand(newSimpleActionCmd("start", "Start a container", runStart))
	ctCmd.AddCommand(newSimpleActionCmd("stop", "Stop a container", runStop))
	ctCmd.AddCommand(newSimpleActionCmd("reboot", "Reboot a container", runReboot))

	// Every other multi-verb resource (backups, snapshots) nests all of
	// its verbs — including creation — under the plural group command
	// (`ct backups create`/`ct snapshots create`), rather than exposing
	// creation as a bare top-level `ct backup`/`ct snapshot`. Keep that
	// pattern consistent here: there is no top-level `ct backup`/
	// `ct snapshot` command.
	backupsCmd := &cobra.Command{
		Use:   "backups",
		Short: "Manage a container's backups",
	}
	ctBackupCreateCmd := newSimpleActionCmd("create", "Create a backup", runBackup)
	ctBackupCreateCmd.Flags().StringVar(&ctBackupStorage, "storage", "", "backup storage target (skips the interactive prompt when set, along with the name-or-vmid argument)")
	backupsCmd.AddCommand(ctBackupCreateCmd)
	backupsCmd.AddCommand(newSimpleActionCmd("list", "List a container's backups", runBackups))
	ctBackupsDeleteCmd := newSimpleActionCmd("delete", "Delete one of a container's backups", runDeleteBackupAction)
	ctBackupsDeleteCmd.Flags().StringVar(&ctBackupsDeleteVolID, "volid", "", "backup volid to delete (skips the interactive listing/prompt when set, along with the name-or-vmid argument)")
	ctBackupsDeleteCmd.Flags().BoolVarP(&ctBackupsDeleteYes, "yes", "y", false, "skip the confirmation prompt")
	backupsCmd.AddCommand(ctBackupsDeleteCmd)

	ctBackupRestoreCmd := &cobra.Command{
		Use:               "restore [name-or-vmid]",
		Short:             "Restore a container from a backup",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeContainerNames,
		RunE:              runCtBackupsRestore,
	}
	ctBackupRestoreCmd.Flags().StringVar(&ctRestoreVolID, "volid", "", "backup volid to restore (skips the interactive listing/prompt when set)")
	ctBackupRestoreCmd.Flags().StringVar(&ctRestoreStorage, "storage", "", "target storage for the restored container (skips the interactive prompt when set)")
	ctBackupRestoreCmd.Flags().BoolVarP(&ctRestoreYes, "yes", "y", false, "skip the confirmation prompt")
	ctBackupRestoreCmd.Flags().StringVar(&ctRestoreNode, "node", "", "restore from any backup on this node instead of an existing container's own backups (disaster recovery — cannot be combined with a name-or-vmid argument)")
	ctBackupRestoreCmd.Flags().IntVar(&ctRestoreVMID, "vmid", 0, "target vmid for --node mode (default: the vmid recorded in the chosen backup)")
	backupsCmd.AddCommand(ctBackupRestoreCmd)

	ctCmd.AddCommand(backupsCmd)

	snapshotsCmd := &cobra.Command{
		Use:   "snapshots",
		Short: "Manage a container's snapshots",
	}
	ctSnapshotCreateCmd := newSimpleActionCmd("create", "Create a snapshot", runSnapshot)
	ctSnapshotCreateCmd.Flags().StringVar(&ctSnapshotName, "snapshot-name", "", "snapshot name (skips the interactive prompt when set, along with the name-or-vmid argument)")
	snapshotsCmd.AddCommand(ctSnapshotCreateCmd)
	snapshotsCmd.AddCommand(newSimpleActionCmd("list", "List a container's snapshots", runSnapshotsAction))
	ctSnapshotsDeleteCmd := newSimpleActionCmd("delete", "Delete one of a container's snapshots", runDeleteSnapshotAction)
	ctSnapshotsDeleteCmd.Flags().StringVar(&ctSnapshotsDeleteName, "snapshot-name", "", "snapshot name to delete (skips the interactive listing/prompt when set, along with the name-or-vmid argument)")
	ctSnapshotsDeleteCmd.Flags().BoolVarP(&ctSnapshotsDeleteYes, "yes", "y", false, "skip the confirmation prompt")
	snapshotsCmd.AddCommand(ctSnapshotsDeleteCmd)
	ctSnapshotsRollbackCmd := newSimpleActionCmd("rollback", "Roll back a container to one of its snapshots", runRollbackSnapshotAction)
	ctSnapshotsRollbackCmd.Flags().StringVar(&ctSnapshotsRollbackName, "snapshot-name", "", "snapshot name to roll back to (skips the interactive listing/prompt when set, along with the name-or-vmid argument)")
	ctSnapshotsRollbackCmd.Flags().BoolVarP(&ctSnapshotsRollbackYes, "yes", "y", false, "skip the confirmation prompt")
	snapshotsCmd.AddCommand(ctSnapshotsRollbackCmd)
	ctCmd.AddCommand(snapshotsCmd)
}
