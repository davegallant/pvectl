package cmd

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/davegallant/pvectl/internal/api"
)

// runVMSummary is runSummary's mirror for QEMU VMs — see its comment.
// Status/current and config are core (hard error on failure); interfaces
// (via the guest agent) and HA state are best-effort, same rationale as
// the container side.
func runVMSummary(client *api.Client, v api.VM) error {
	ctx := context.Background()

	status, err := client.QemuStatus(ctx, v.Node, v.VMID)
	if err != nil {
		return fmt.Errorf("fetching status for %s (%d): %w", v.Name, v.VMID, err)
	}
	config, err := client.GetVMConfig(ctx, v.Node, v.VMID)
	if err != nil {
		return fmt.Errorf("fetching config for %s (%d): %w", v.Name, v.VMID, err)
	}

	interfaces, _ := client.QemuInterfaces(ctx, v.Node, v.VMID)
	haState, haManaged, _ := client.HAResourceState(ctx, fmt.Sprintf("vm:%d", v.VMID))

	fmt.Print(renderVMSummary(v, status, config, interfaces, haState, haManaged))
	return nil
}

// renderVMSummary is renderSummary's mirror for QEMU VMs. It swaps the
// container side's Unprivileged line for Guest Agent (whether the config
// enables it) — a VM has no unprivileged concept — and skips a SWAP usage
// line, since QEMU's status/current has no swap/maxswap pair the way
// LXC's does. It performs no I/O, so it's directly unit-testable, same
// pattern as renderSummary.
func renderVMSummary(v api.VM, status api.VMStatus, config api.VMConfig, interfaces []api.QemuInterface, haState string, haManaged bool) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "VM %d (%s) on node %q\n\n", v.VMID, v.Name, v.Node)

	ha := "none"
	if haManaged {
		ha = haState
	}
	agent := "no"
	if strings.HasPrefix(config.Fields["agent"], "1") {
		agent = "yes"
	}

	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "Status\t%s\n", status.Status)
	_, _ = fmt.Fprintf(tw, "HA State\t%s\n", ha)
	_, _ = fmt.Fprintf(tw, "Node\t%s\n", v.Node)
	_, _ = fmt.Fprintf(tw, "Guest Agent\t%s\n", agent)
	_, _ = fmt.Fprintln(tw)
	_, _ = fmt.Fprintf(tw, "CPU usage\t%s\n", formatCPUUsage(status.CPU, status.CPUs))
	_, _ = fmt.Fprintf(tw, "Memory usage\t%s\n", formatUsage(status.Mem, status.MaxMem))
	_, _ = fmt.Fprintf(tw, "Bootdisk size\t%s\n", formatUsage(status.Disk, status.MaxDisk))
	_ = tw.Flush()

	var ips []string
	for _, iface := range interfaces {
		ips = append(ips, iface.IPAddresses...)
	}
	if len(ips) > 0 {
		_, _ = fmt.Fprintln(&buf)
		_, _ = fmt.Fprintln(&buf, "IPs:")
		for _, ip := range ips {
			_, _ = fmt.Fprintf(&buf, "  %s\n", ip)
		}
	}

	return buf.String()
}

func init() {
	qmCmd.AddCommand(newSimpleVMActionCmd("summary", "Show a VM's status, HA state, resource usage, and IPs", runVMSummary))
}
