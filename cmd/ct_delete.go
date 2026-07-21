package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/davegallant/pvectl/internal/api"
)

// ctDeleteYes/ctDeletePurge/ctDeleteForce back `ct delete`'s `-y`/`--yes`,
// `--purge`, and `-f`/`--force` flags, matching
// ctBackupsDeleteYes/ctSnapshotsDeleteYes's convention.
var ctDeleteYes bool
var ctDeletePurge bool
var ctDeleteForce bool

// runDeleteContainer requires the user to type "yes" before permanently
// destroying node/vmid — there is no undo, so this deliberately doesn't
// accept a bare "y" or default to yes, unless skipConfirm is set (the
// `-y`/`--yes` flag), same discipline as runDeleteBackup/
// runDeleteSnapshot. Deletion is a Proxmox task like start/stop/backup/
// migrate, so it runs through the same live spinner via
// runProgressAction rather than finishing silently.
func runDeleteContainer(client *api.Client, node string, vmid int, name string, purge bool, force bool, skipConfirm bool) error {
	fmt.Printf("about to permanently delete container %s (%d) — this cannot be undone\n", name, vmid)
	if !skipConfirm {
		fmt.Print("type 'yes' to confirm: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if strings.TrimSpace(line) != "yes" {
			fmt.Println("aborted, container not deleted")
			return nil
		}
	}

	upid, err := client.DeleteContainer(context.Background(), node, vmid, purge, force)
	if err != nil {
		return fmt.Errorf("deleting container %s (%d): %w", name, vmid, err)
	}
	return runProgressAction(client, node, upid,
		fmt.Sprintf("deleting container %s (%d)", name, vmid),
		fmt.Sprintf("deleted container %s (%d)", name, vmid))
}

// runDeleteAction adapts runDeleteContainer to newSimpleActionCmd's
// func(*api.Client, api.Container) error shape, reading the package-level
// flag vars the same way runDeleteBackupAction/runDeleteSnapshotAction do.
func runDeleteAction(client *api.Client, c api.Container) error {
	return runDeleteContainer(client, c.Node, c.VMID, c.Name, ctDeletePurge, ctDeleteForce, ctDeleteYes)
}

func init() {
	ctDeleteCmd := newSimpleActionCmd("destroy", "Permanently delete a container", runDeleteAction)
	ctDeleteCmd.Aliases = []string{"delete"}
	ctDeleteCmd.Flags().BoolVarP(&ctDeleteYes, "yes", "y", false, "skip the confirmation prompt")
	ctDeleteCmd.Flags().BoolVar(&ctDeletePurge, "purge", false, "also remove the container from backup jobs, replication jobs, HA, and ACLs")
	ctDeleteCmd.Flags().BoolVarP(&ctDeleteForce, "force", "f", false, "force destroy even if the container is running")
	ctCmd.AddCommand(ctDeleteCmd)
}
