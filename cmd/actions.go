package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/davegallant/pvectl/internal/tui"
	"github.com/spf13/cobra"
)

// actionAnnouncements maps each action-menu leaf (tui.ActionTree's Action
// values, matching dispatchAction/dispatchVMAction's switch cases) to a
// human-readable present-participle phrase for runCt/runQm's pre-dispatch
// announcement line — "Snapshotting web01 (101)" instead of the raw
// internal key "snapshot: web01 (101)". Keep in sync with
// dispatchAction/dispatchVMAction: every case there needs an entry here
// (TestActionAnnouncementsCoverActionTree checks this).
var actionAnnouncements = map[string]string{
	"enter":             "Entering",
	"start":             "Starting",
	"stop":              "Stopping",
	"reboot":            "Rebooting",
	"edit":              "Editing",
	"rename":            "Renaming",
	"snapshot":          "Snapshotting",
	"snapshots":         "Listing snapshots of",
	"delete-snapshot":   "Deleting a snapshot of",
	"rollback-snapshot": "Rolling back",
	"backup":            "Backing up",
	"backups":           "Listing backups of",
	"delete-backup":     "Deleting a backup of",
	"migrate":           "Migrating",
	"delete":            "Deleting",
}

// announceAction prints a human-readable "<Verb-ing> <name> (<vmid>)" line
// before runCt/runQm dispatch the action-menu selection. Falls back to the
// raw action key if it's ever missing from actionAnnouncements (shouldn't
// happen — see TestActionAnnouncementsCoverActionTree — but a slightly
// ugly line beats a blank one).
func announceAction(action, name string, vmid int) {
	verb, ok := actionAnnouncements[action]
	if !ok {
		verb = action
	}
	fmt.Printf("%s %s (%d)\n", verb, name, vmid)
}

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
// the interactive name prompt — set only when the `ct snapshots create`
// subcommand registers it, so the `ct select` menu's snapshot action
// still always prompts.
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
// skips the interactive storage prompt — set only when the
// `ct backups create` subcommand registers it, so the `ct select` menu's
// backup action still always prompts.
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
// migrates c — used both by the `ct select` action menu (dispatchAction,
// which already has a container in hand) and by bare `ct migrate`
// (runMigrateAction, which picks one first). The direct
// `ct migrate <name-or-vmid> --target <node>` form (ctMigrateCmd in
// migrate.go) resolves the target some other way and calls runMigrate
// directly instead, so it never touches stdin.
func runMigrateWithPrompt(client *api.Client, c api.Container) error {
	printRestartNotice(c)
	target, err := promptTargetNode(client, c.Node)
	if err != nil {
		return err
	}
	return runMigrate(client, c, target)
}

// runMigrateAction is the bare `ct migrate` entry point (no
// name-or-vmid argument): picks a container via the fuzzy picker, then
// delegates to runMigrateWithPrompt.
func runMigrateAction(client *api.Client) error {
	c, err := selectContainer(client)
	if err != nil {
		if errors.Is(err, tui.ErrCancelled) {
			return nil
		}
		return err
	}
	return runMigrateWithPrompt(client, c)
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
// entirely, making deletion scriptable. They stay at their zero values
// (and the interactive flow stays required) when the action is reached
// via the `ct select` menu instead, since only the direct delete
// subcommands register the flags.
var ctBackupsDeleteVolID string
var ctBackupsDeleteYes bool
var ctSnapshotsDeleteName string
var ctSnapshotsDeleteYes bool
var ctSnapshotsRollbackName string
var ctSnapshotsRollbackYes bool

func runDeleteBackupAction(client *api.Client, c api.Container) error {
	return runDeleteBackup(client, c.Node, c.VMID, c.Name, ctBackupsDeleteVolID, ctBackupsDeleteYes)
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

// newSimpleActionCmd builds a `ct <use> [name-or-vmid]` command: given a
// name or vmid, it acts directly on that container (via
// resolveContainer/findContainer), never touching the interactive
// picker; given none, it falls back to the fuzzy picker as before.
func newSimpleActionCmd(use, short string, run func(*api.Client, api.Container) error) *cobra.Command {
	return &cobra.Command{
		Use:   use + " [name-or-vmid]",
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
