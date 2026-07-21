package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/davegallant/pvectl/internal/api"
)

// qmDeleteYes/qmDeletePurge back `qm delete`'s `-y`/`--yes` and `--purge`
// flags — set only when the direct `qm delete` subcommand registers
// them, mirroring ctDeleteYes/ctDeletePurge. There's no `--force` mirror:
// Proxmox's DELETE .../qemu/{vmid} has no equivalent to the LXC
// endpoint's force-destroy-while-running param (see api.DeleteVM).
var qmDeleteYes bool
var qmDeletePurge bool

// runDeleteVM is runDeleteContainer's mirror for QEMU VMs — see its
// comment. There's no force parameter to thread through (api.DeleteVM
// has none), so a running VM must be stopped first, same as an LXC
// container deleted without --force.
func runDeleteVM(client *api.Client, node string, vmid int, name string, purge bool, skipConfirm bool) error {
	fmt.Printf("about to permanently delete VM %s (%d) — this cannot be undone\n", name, vmid)
	if !skipConfirm {
		fmt.Print("type 'yes' to confirm: ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if strings.TrimSpace(line) != "yes" {
			fmt.Println("aborted, VM not deleted")
			return nil
		}
	}

	upid, err := client.DeleteVM(context.Background(), node, vmid, purge)
	if err != nil {
		return fmt.Errorf("deleting VM %s (%d): %w", name, vmid, err)
	}
	return runProgressAction(client, node, upid,
		fmt.Sprintf("deleting VM %s (%d)", name, vmid),
		fmt.Sprintf("deleted VM %s (%d)", name, vmid))
}

// runDeleteVMAction adapts runDeleteVM to newSimpleVMActionCmd's
// func(*api.Client, api.VM) error shape, reading the package-level flag
// vars the same way runDeleteAction does for containers.
func runDeleteVMAction(client *api.Client, v api.VM) error {
	return runDeleteVM(client, v.Node, v.VMID, v.Name, qmDeletePurge, qmDeleteYes)
}

func init() {
	qmDeleteCmd := newSimpleVMActionCmd("destroy", "Permanently delete a VM", mutationDestructive, runDeleteVMAction)
	qmDeleteCmd.Aliases = []string{"delete"}
	qmDeleteCmd.Flags().BoolVarP(&qmDeleteYes, "yes", "y", false, "skip the confirmation prompt")
	qmDeleteCmd.Flags().BoolVar(&qmDeletePurge, "purge", false, "also remove the VM from backup jobs, replication jobs, HA, and ACLs")
	qmCmd.AddCommand(qmDeleteCmd)
}
