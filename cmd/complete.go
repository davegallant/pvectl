package cmd

import (
	"context"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/spf13/cobra"
)

// containerNames extracts the completion candidates from an already-fetched
// container list — split out from completeContainerNames so the mapping is
// testable without a live client.
func containerNames(containers []api.Container) []string {
	names := make([]string, 0, len(containers))
	for _, c := range containers {
		names = append(names, c.Name)
	}
	return names
}

// vmNames is containerNames's mirror for QEMU VMs.
func vmNames(vms []api.VM) []string {
	names := make([]string, 0, len(vms))
	for _, v := range vms {
		names = append(names, v.Name)
	}
	return names
}

// completeContainerNames is the ValidArgsFunction shared by every
// `ct <verb> [name-or-vmid]` command (wired in via newSimpleActionCmd, plus
// ct select/edit/enter/migrate). It fetches the live container list on
// every invocation — no caching, same tradeoff kubectl makes for
// `kubectl get pod <TAB>` — so completion is always accurate but pays a
// network round trip per keystroke. Only the first positional arg is
// completable; once it's filled in there's nothing left to suggest.
func completeContainerNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	client, err := loadClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	containers, err := client.ListContainers(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return containerNames(containers), cobra.ShellCompDirectiveNoFileComp
}

// completeVMNames is completeContainerNames's mirror for QEMU VMs.
func completeVMNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	client, err := loadClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	vms, err := client.ListVMs(context.Background())
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return vmNames(vms), cobra.ShellCompDirectiveNoFileComp
}
