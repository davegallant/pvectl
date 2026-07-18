package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/davegallant/pvectl/internal/api"
)

// runListSnapshotsVM is runListSnapshots's mirror for QEMU VMs.
func runListSnapshotsVM(client *api.Client, node string, vmid int, name string) error {
	snapshots, err := client.ListSnapshotsVM(context.Background(), node, vmid)
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

// runDeleteSnapshotVM is runDeleteSnapshot's mirror for QEMU VMs.
func runDeleteSnapshotVM(client *api.Client, node string, vmid int, name string, snapName string, skipConfirm bool) error {
	snapshots, err := client.ListSnapshotsVM(context.Background(), node, vmid)
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

	upid, err := client.DeleteSnapshotVM(context.Background(), node, vmid, snapName)
	if err != nil {
		return fmt.Errorf("deleting snapshot %q: %w", snapName, err)
	}
	return runProgressAction(client, node, upid,
		fmt.Sprintf("deleting snapshot %q of %s (%d)", snapName, name, vmid),
		fmt.Sprintf("deleted snapshot %q of %s (%d)", snapName, name, vmid))
}

// runRollbackSnapshotVM is runRollbackSnapshot's mirror for QEMU VMs.
func runRollbackSnapshotVM(client *api.Client, node string, vmid int, name string, snapName string, skipConfirm bool) error {
	snapshots, err := client.ListSnapshotsVM(context.Background(), node, vmid)
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

	upid, err := client.RollbackVM(context.Background(), node, vmid, snapName)
	if err != nil {
		return fmt.Errorf("rolling back to snapshot %q: %w", snapName, err)
	}
	return runProgressAction(client, node, upid,
		fmt.Sprintf("rolling back %s (%d) to snapshot %q", name, vmid, snapName),
		fmt.Sprintf("rolled back %s (%d) to snapshot %q", name, vmid, snapName))
}
