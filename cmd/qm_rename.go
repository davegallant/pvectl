package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/davegallant/pvectl/internal/api"
)

// qmRenameName is ctRenameName's mirror for QEMU VMs — see its comment.
var qmRenameName string

// runRenameVM is runRename's mirror for QEMU VMs — it updates the "name"
// config field instead of "hostname", since that's the field Proxmox
// surfaces as a VM's display name.
func runRenameVM(client *api.Client, v api.VM) error {
	newName := qmRenameName
	if newName == "" {
		fmt.Print("new name: ")
		reader := bufio.NewReader(os.Stdin)
		nameLine, _ := reader.ReadString('\n')
		newName = strings.TrimSpace(nameLine)
	}
	if newName == "" {
		return fmt.Errorf("new name required")
	}
	return renameVM(client, v, newName)
}

// renameVM is renameContainer's mirror for QEMU VMs — split out from
// runRenameVM for the same reason (directly unit-testable without stdin).
func renameVM(client *api.Client, v api.VM, newName string) error {
	ctx := context.Background()
	cfg, err := client.GetVMConfig(ctx, v.Node, v.VMID)
	if err != nil {
		return fmt.Errorf("fetching config: %w", err)
	}

	err = client.PutVMConfig(ctx, v.Node, v.VMID, map[string]string{"name": newName}, cfg.Digest)
	if errors.Is(err, api.ErrDigestMismatch) {
		return fmt.Errorf("config changed elsewhere, re-run rename")
	}
	if err != nil {
		return fmt.Errorf("renaming %s (%d): %w", v.Name, v.VMID, err)
	}

	fmt.Printf("renamed %s (%d) to %s\n", v.Name, v.VMID, newName)
	return nil
}

func init() {
	qmRenameCmd := newSimpleVMActionCmd("rename", "Rename a VM", runRenameVM)
	qmRenameCmd.Flags().StringVar(&qmRenameName, "name", "", "new name (skips the interactive prompt when set, along with the name-or-vmid argument)")
	qmCmd.AddCommand(qmRenameCmd)
}
