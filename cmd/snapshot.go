package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/davegallant/pvectl/internal/api"
)

// runListSnapshots prints a NAME/DESCRIPTION/DATE table of vmid's LXC
// container snapshots (newest first).
func runListSnapshots(client *api.Client, node string, vmid int, name string) error {
	snapshots, err := client.ListSnapshots(context.Background(), node, vmid)
	if err != nil {
		return fmt.Errorf("listing snapshots for %s (%d): %w", name, vmid, err)
	}
	if len(snapshots) == 0 {
		fmt.Printf("no snapshots found for %s (%d)\n", name, vmid)
		return nil
	}
	fmt.Print(renderSnapshots(snapshots))
	return nil
}

// runDeleteSnapshot lists vmid's snapshots, then either uses snapName
// directly (the `--name` flag) or prompts for it interactively, and
// requires the user to type "yes" before permanently deleting it — there
// is no undo, so this deliberately doesn't accept a bare "y" or default
// to yes, unless skipConfirm is set (the `-y`/`--yes` flag). Only a name
// that actually appeared in the listing is accepted, so a typo can't be
// sent straight to the delete API. Passing both --name and -y makes
// deletion fully non-interactive (no stdin reads), for scripting.
// Deletion is a Proxmox task like start/stop/backup/migrate, so it runs
// through the same live spinner via runProgressAction rather than
// finishing silently the way DeleteBackup does.
func runDeleteSnapshot(client *api.Client, node string, vmid int, name string, snapName string, skipConfirm bool) error {
	snapshots, err := client.ListSnapshots(context.Background(), node, vmid)
	if err != nil {
		return fmt.Errorf("listing snapshots for %s (%d): %w", name, vmid, err)
	}
	if len(snapshots) == 0 {
		fmt.Printf("no snapshots found for %s (%d)\n", name, vmid)
		return nil
	}

	reader := bufio.NewReader(os.Stdin)

	if snapName == "" {
		fmt.Print(renderSnapshots(snapshots))
		fmt.Print("snapshot to delete: ")
		nameLine, _ := reader.ReadString('\n')
		snapName = strings.TrimSpace(nameLine)
	}

	var found bool
	for _, s := range snapshots {
		if s.Name == snapName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("no snapshot named %q found for %s (%d)", snapName, name, vmid)
	}

	fmt.Printf("about to permanently delete snapshot %q of %s (%d) — this cannot be undone\n", snapName, name, vmid)
	if !skipConfirm {
		fmt.Print("type 'yes' to confirm: ")
		confirmLine, _ := reader.ReadString('\n')
		if strings.TrimSpace(confirmLine) != "yes" {
			fmt.Println("aborted, snapshot not deleted")
			return nil
		}
	}

	upid, err := client.DeleteSnapshot(context.Background(), node, vmid, snapName)
	if err != nil {
		return fmt.Errorf("deleting snapshot %q: %w", snapName, err)
	}
	return runProgressAction(client, node, upid,
		fmt.Sprintf("deleting snapshot %q of %s (%d)", snapName, name, vmid),
		fmt.Sprintf("deleted snapshot %q of %s (%d)", snapName, name, vmid))
}

// runRollbackSnapshot lists vmid's snapshots, then either uses snapName
// directly (the `--name` flag) or prompts for it interactively, and
// requires the user to type "yes" before rolling back — rolling back
// discards every change made since the snapshot was taken, which cannot
// be undone, so this deliberately doesn't accept a bare "y" or default
// to yes, unless skipConfirm is set (the `-y`/`--yes` flag). Only a name
// that actually appeared in the listing is accepted, so a typo can't be
// sent straight to the rollback API. Passing both --name and -y makes
// the rollback fully non-interactive (no stdin reads), for scripting.
// Rollback is a Proxmox task like start/stop/delete/migrate, so it runs
// through the same live spinner via runProgressAction.
func runRollbackSnapshot(client *api.Client, node string, vmid int, name string, snapName string, skipConfirm bool) error {
	snapshots, err := client.ListSnapshots(context.Background(), node, vmid)
	if err != nil {
		return fmt.Errorf("listing snapshots for %s (%d): %w", name, vmid, err)
	}
	if len(snapshots) == 0 {
		fmt.Printf("no snapshots found for %s (%d)\n", name, vmid)
		return nil
	}

	reader := bufio.NewReader(os.Stdin)

	if snapName == "" {
		fmt.Print(renderSnapshots(snapshots))
		fmt.Print("snapshot to roll back to: ")
		nameLine, _ := reader.ReadString('\n')
		snapName = strings.TrimSpace(nameLine)
	}

	var found bool
	for _, s := range snapshots {
		if s.Name == snapName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("no snapshot named %q found for %s (%d)", snapName, name, vmid)
	}

	fmt.Printf("about to roll back %s (%d) to snapshot %q — this discards all changes made since, and cannot be undone\n", name, vmid, snapName)
	if !skipConfirm {
		fmt.Print("type 'yes' to confirm: ")
		confirmLine, _ := reader.ReadString('\n')
		if strings.TrimSpace(confirmLine) != "yes" {
			fmt.Println("aborted, not rolled back")
			return nil
		}
	}

	upid, err := client.Rollback(context.Background(), node, vmid, snapName)
	if err != nil {
		return fmt.Errorf("rolling back to snapshot %q: %w", snapName, err)
	}
	return runProgressAction(client, node, upid,
		fmt.Sprintf("rolling back %s (%d) to snapshot %q", name, vmid, snapName),
		fmt.Sprintf("rolled back %s (%d) to snapshot %q", name, vmid, snapName))
}

// renderSnapshots formats a snapshot listing from already-fetched data.
// It performs no I/O, so it's directly unit-testable — shared by both
// `ct`/`qm` since api.Snapshot doesn't carry guest type.
func renderSnapshots(snapshots []api.Snapshot) string {
	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tDESCRIPTION\tDATE")
	for _, s := range snapshots {
		// Descriptions are free-form Proxmox user text and can contain
		// newlines (e.g. set from the web UI) — an embedded newline in a
		// tabwriter cell would break that row's column alignment, so
		// collapse it to spaces first.
		description := strings.Join(strings.Fields(s.Description), " ")
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Name, description, time.Unix(s.SnapTime, 0).Format("2006-01-02 15:04"))
	}
	_ = tw.Flush()
	return buf.String()
}
