package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/davegallant/pvectl/internal/api"
)

// promptStorage prompts for a vzdump destination storage, listing the
// storage names available on node (if they can be fetched) so the user
// doesn't have to already know Proxmox's storage IDs by heart. Falls back
// to a bare prompt if the listing fetch fails — that's a convenience, not
// something worth failing the backup over.
func promptStorage(client *api.Client, node string) string {
	if names := storageNamesForNode(client, node); len(names) > 0 {
		fmt.Printf("storage (%s): ", strings.Join(names, ", "))
	} else {
		fmt.Print("storage: ")
	}

	reader := bufio.NewReader(os.Stdin)
	storage, _ := reader.ReadString('\n')
	return strings.TrimSpace(storage)
}

func storageNamesForNode(client *api.Client, node string) []string {
	resources, err := client.ClusterResources(context.Background())
	if err != nil {
		return nil
	}

	var names []string
	for _, s := range resources.Storage {
		if s.Node == node {
			names = append(names, s.Name)
		}
	}
	sort.Strings(names)
	return names
}

// fetchBackups looks up every storage mounted on node and lists vzdump
// backups for vmid across all of them, newest first.
func fetchBackups(client *api.Client, node string, vmid int) ([]api.Backup, error) {
	storages := storageNamesForNode(client, node)
	return client.ListBackups(context.Background(), node, storages, vmid)
}

// runListBackups prints a VOLID/STORAGE/SIZE/DATE table of vmid's backups
// (newest first).
func runListBackups(client *api.Client, node string, vmid int, name string) error {
	backups, err := fetchBackups(client, node, vmid)
	if err != nil {
		return fmt.Errorf("listing backups for %s (%d): %w", name, vmid, err)
	}
	if len(backups) == 0 {
		fmt.Printf("no backups found for %s (%d)\n", name, vmid)
		return nil
	}
	fmt.Print(renderBackups(backups))
	return nil
}

// runDeleteBackup lists vmid's backups, then either uses volid directly
// (the `--volid` flag) or prompts for it interactively, and requires the
// user to type "yes" before permanently deleting it — there is no undo,
// so this deliberately doesn't accept a bare "y" or default to yes,
// unless skipConfirm is set (the `-y`/`--yes` flag). Only volids that
// actually appeared in the listing are accepted, so a typo can't be sent
// straight to the delete API. Passing both --volid and -y makes deletion
// fully non-interactive (no stdin reads), for scripting.
func runDeleteBackup(client *api.Client, node string, vmid int, name string, volid string, skipConfirm bool) error {
	backups, err := fetchBackups(client, node, vmid)
	if err != nil {
		return fmt.Errorf("listing backups for %s (%d): %w", name, vmid, err)
	}
	if len(backups) == 0 {
		fmt.Printf("no backups found for %s (%d)\n", name, vmid)
		return nil
	}

	reader := bufio.NewReader(os.Stdin)

	if volid == "" {
		fmt.Print(renderBackups(backups))
		fmt.Print("volid to delete: ")
		volidLine, _ := reader.ReadString('\n')
		volid = strings.TrimSpace(volidLine)
	}

	target := findBackup(backups, volid)
	if target == nil {
		return fmt.Errorf("no backup with volid %q found for %s (%d)", volid, name, vmid)
	}

	fmt.Printf("about to permanently delete %s (%s, %s) — this cannot be undone\n",
		target.VolID, formatBytes(target.Size), time.Unix(target.CTime, 0).Format("2006-01-02 15:04"))
	if !skipConfirm {
		fmt.Print("type 'yes' to confirm: ")
		confirmLine, _ := reader.ReadString('\n')
		if strings.TrimSpace(confirmLine) != "yes" {
			fmt.Println("aborted, backup not deleted")
			return nil
		}
	}

	if err := client.DeleteBackup(context.Background(), node, target.Storage, target.VolID); err != nil {
		return fmt.Errorf("deleting backup %s: %w", target.VolID, err)
	}
	fmt.Printf("deleted %s\n", target.VolID)
	return nil
}

// renderBackups formats a backup listing from already-fetched data. It
// performs no I/O, so it's directly unit-testable.
func renderBackups(backups []api.Backup) string {
	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "VOLID\tVMID\tSTORAGE\tSIZE\tDATE")
	for _, b := range backups {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n", b.VolID, b.VMID, b.Storage, formatBytes(b.Size), time.Unix(b.CTime, 0).Format("2006-01-02 15:04"))
	}
	_ = tw.Flush()
	return buf.String()
}

// vzdumpGuestPrefix maps a backup's guest type ("lxc"/"qemu") to the
// filename prefix Proxmox's own vzdump uses
// (vzdump-lxc-<vmid>-.../vzdump-qemu-<vmid>-...) — used to filter a
// node-wide backup listing (fetchAllBackups) down to the right guest
// type, so `qm backups restore`'s disaster-recovery browser never offers
// a container's backup and vice versa.
func vzdumpGuestPrefix(guestType string) string {
	return "vzdump-" + guestType + "-"
}

// filterBackupsByGuestType keeps only backups whose volid matches
// guestType's vzdump filename prefix. The volid's path segment (after the
// storage's "storage:" prefix) is what's checked, e.g.
// "local:backup/vzdump-lxc-101-2024_01_01-00_00_00.tar.zst".
func filterBackupsByGuestType(backups []api.Backup, guestType string) []api.Backup {
	prefix := vzdumpGuestPrefix(guestType)
	var out []api.Backup
	for _, b := range backups {
		if idx := strings.Index(b.VolID, ":"); idx >= 0 && strings.Contains(b.VolID[idx+1:], prefix) {
			out = append(out, b)
		}
	}
	return out
}

// fetchAllBackups looks up every storage mounted on node and lists every
// vzdump backup found across all of them, regardless of vmid — the
// disaster-recovery counterpart to fetchBackups, used when the original
// guest may no longer exist to filter by.
func fetchAllBackups(client *api.Client, node string) ([]api.Backup, error) {
	storages := storageNamesForNode(client, node)
	return client.ListAllBackups(context.Background(), node, storages)
}

// restoreFunc matches Client.RestoreContainer's and Client.RestoreVM's
// identical signatures, letting the shared restore helpers below take
// either as a plain func value.
type restoreFunc func(ctx context.Context, node string, vmid int, archive, storage string, force bool) (string, error)

// promptVolID renders backups and prompts for the volid to restore,
// mirroring runDeleteBackup's listing/prompt pair.
func promptVolID(backups []api.Backup) string {
	fmt.Print(renderBackups(backups))
	fmt.Print("volid to restore: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// findBackup returns the backup in backups whose VolID matches volid, so
// a typo can't be sent straight to the restore API — same discipline as
// runDeleteBackup's volid lookup.
func findBackup(backups []api.Backup, volid string) *api.Backup {
	for i := range backups {
		if backups[i].VolID == volid {
			return &backups[i]
		}
	}
	return nil
}

// confirmOverwrite requires the user to type "yes" before restore
// overwrites vmid's current state — there is no undo, same discipline as
// runDeleteBackup/runDeleteContainer. skipConfirm (-y/--yes) bypasses the
// prompt for scripting. label identifies the guest being overwritten
// (e.g. "container web01" or just "container" when there's no name to
// show, as in disaster recovery).
func confirmOverwrite(label string, vmid int, skipConfirm bool) bool {
	fmt.Printf("about to overwrite %s (%d) with a restored backup — this cannot be undone\n", label, vmid)
	if skipConfirm {
		return true
	}
	fmt.Print("type 'yes' to confirm: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line) == "yes"
}

// runRestoreBackup restores vmid (an existing guest) from one of its own
// backups (fetchBackups) — the in-place restore path shared by
// runRestoreBackupAction/runRestoreBackupVMAction and by the direct
// `ct backups restore`/`qm backups restore [name-or-vmid]` commands.
// Since vmid already exists by definition, this always sends force=1 to
// the restore API; the "type yes" confirmation (confirmOverwrite) is the
// safety net instead. kind labels the guest type in messages
// ("container"/"VM"); restore is Client.RestoreContainer or
// Client.RestoreVM.
func runRestoreBackup(client *api.Client, node string, vmid int, name, kind, volid, storage string, skipConfirm bool, restore restoreFunc) error {
	backups, err := fetchBackups(client, node, vmid)
	if err != nil {
		return fmt.Errorf("listing backups for %s (%d): %w", name, vmid, err)
	}
	if len(backups) == 0 {
		return fmt.Errorf("no backups found for %s (%d)", name, vmid)
	}

	if volid == "" {
		volid = promptVolID(backups)
	}
	target := findBackup(backups, volid)
	if target == nil {
		return fmt.Errorf("no backup with volid %q found for %s (%d)", volid, name, vmid)
	}

	if !confirmOverwrite(fmt.Sprintf("%s %s", kind, name), vmid, skipConfirm) {
		fmt.Println("aborted, not restored")
		return nil
	}

	upid, err := restore(context.Background(), node, vmid, target.VolID, storage, true)
	if err != nil {
		return fmt.Errorf("restoring %s (%d): %w", name, vmid, err)
	}
	return runProgressAction(client, node, upid,
		fmt.Sprintf("restoring %s (%d)", name, vmid),
		fmt.Sprintf("restored %s (%d)", name, vmid))
}

// runRestoreFromNode restores a backup found anywhere on node (not
// scoped to any one guest's own backups) — the disaster-recovery path
// for a guest that no longer exists to be resolved via the picker, used
// by `ct backups restore --node`/`qm backups restore --node`. backups is
// pre-filtered to the right guest type (filterBackupsByGuestType) by the
// caller. targetVMID, when non-zero, overrides the vmid recorded in the
// chosen backup (e.g. to clone under a new id); zero means "use the
// backup's own vmid". exists reports whether the resolved target vmid is
// already in use (Client.ListContainers/ListVMs) — only then is force
// sent and confirmation required, matching runRestoreBackup's discipline
// but skipping the prompt entirely for a genuinely free vmid, the same
// way `ct create` never asks to confirm creating something new.
func runRestoreFromNode(client *api.Client, node, kind string, backups []api.Backup, targetVMID int, volid, storage string, skipConfirm bool, restore restoreFunc, exists func(vmid int) (bool, error)) error {
	if len(backups) == 0 {
		return fmt.Errorf("no %s backups found on node %s", kind, node)
	}

	if volid == "" {
		volid = promptVolID(backups)
	}
	target := findBackup(backups, volid)
	if target == nil {
		return fmt.Errorf("no %s backup with volid %q found on node %s", kind, volid, node)
	}

	vmid := targetVMID
	if vmid == 0 {
		vmid = target.VMID
	}

	overwriting, err := exists(vmid)
	if err != nil {
		return fmt.Errorf("checking whether vmid %d already exists: %w", vmid, err)
	}
	if overwriting {
		if !confirmOverwrite(kind, vmid, skipConfirm) {
			fmt.Println("aborted, not restored")
			return nil
		}
	}

	upid, err := restore(context.Background(), node, vmid, target.VolID, storage, overwriting)
	if err != nil {
		return fmt.Errorf("restoring %s (%d) from %s: %w", kind, vmid, target.VolID, err)
	}
	return runProgressAction(client, node, upid,
		fmt.Sprintf("restoring %s (%d)", kind, vmid),
		fmt.Sprintf("restored %s (%d)", kind, vmid))
}
