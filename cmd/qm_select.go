package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/davegallant/pvectl/internal/api"
)

// findVM is findContainer's mirror for QEMU VMs.
func findVM(client *api.Client, identifier string) (api.VM, error) {
	vms, err := client.ListVMs(context.Background())
	if err != nil {
		return api.VM{}, fmt.Errorf("listing VMs: %w", err)
	}

	if vmid, err := strconv.Atoi(identifier); err == nil {
		for _, v := range vms {
			if v.VMID == vmid {
				return v, nil
			}
		}
		return api.VM{}, fmt.Errorf("no VM with vmid %d found", vmid)
	}

	var matches []api.VM
	for _, v := range vms {
		if v.Name == identifier {
			matches = append(matches, v)
		}
	}
	switch len(matches) {
	case 0:
		return api.VM{}, fmt.Errorf("no VM named %q found", identifier)
	case 1:
		return matches[0], nil
	default:
		return api.VM{}, fmt.Errorf("multiple VMs named %q found, use its vmid instead", identifier)
	}
}

// vmExists is containerExists's mirror for QEMU VMs.
func vmExists(client *api.Client, vmid int) (bool, error) {
	vms, err := client.ListVMs(context.Background())
	if err != nil {
		return false, fmt.Errorf("listing VMs: %w", err)
	}
	for _, v := range vms {
		if v.VMID == vmid {
			return true, nil
		}
	}
	return false, nil
}

// resolveVM is resolveContainer's mirror for QEMU VMs.
func resolveVM(client *api.Client, args []string) (api.VM, error) {
	if len(args) == 0 {
		return api.VM{}, fmt.Errorf("a VM name or vmid is required")
	}
	return findVM(client, args[0])
}
