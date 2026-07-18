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

	var target *api.Backup
	for i := range backups {
		if backups[i].VolID == volid {
			target = &backups[i]
			break
		}
	}
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
	_, _ = fmt.Fprintln(tw, "VOLID\tSTORAGE\tSIZE\tDATE")
	for _, b := range backups {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", b.VolID, b.Storage, formatBytes(b.Size), time.Unix(b.CTime, 0).Format("2006-01-02 15:04"))
	}
	_ = tw.Flush()
	return buf.String()
}
