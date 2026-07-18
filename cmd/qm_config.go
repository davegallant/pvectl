package cmd

import "github.com/spf13/cobra"

// qmConfigCmd groups every `qm config` subcommand — currently just `edit`
// (qm_edit.go). A package-level var (rather than a local one scoped to
// some other file's init) so qm_edit.go's own init can add to it. Unlike
// ctConfigCmd, there's no `append` here: VMConfig has no raw lxc.*-style
// passthrough block to begin with, so there's nothing Proxmox's API fails
// to expose for QEMU VMs the way it does for LXC containers.
var qmConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage a VM's config",
}

func init() {
	qmCmd.AddCommand(qmConfigCmd)
}
