package cmd

import (
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestRenderVMSummaryRunningVM(t *testing.T) {
	v := api.VM{VMID: 200, Name: "unifi", Node: "pve-g3-1", Status: "running"}
	status := api.VMStatus{
		Status: "running", CPU: 0.0037, CPUs: 4,
		Mem: 1470000000, MaxMem: 6500000000,
		Disk: 0, MaxDisk: 20960000000,
	}
	config := api.VMConfig{Fields: map[string]string{"agent": "1,fstrim_cloned_disks=1"}}
	interfaces := []api.QemuInterface{
		{Name: "eth0", HWAddr: "52:54:00:12:34:56", IPAddresses: []string{"192.168.1.50", "fe80::5054:ff:fe12:3456"}},
	}

	got := renderVMSummary(v, status, config, interfaces, "", false)

	if !strings.HasPrefix(got, `VM 200 (unifi) on node "pve-g3-1"`) {
		t.Errorf("renderVMSummary() = %q, want it to start with the VM header", got)
	}
	for _, want := range []string{
		"Status", "running",
		"HA State", "none",
		"Guest Agent", "yes",
		"0.37% of 4 CPUs",
		"22.62% (1.4G of 6.1G)",
		"192.168.1.50",
		"fe80::5054:ff:fe12:3456",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVMSummary() = %q, want it to contain %q", got, want)
		}
	}
}

func TestRenderVMSummaryHAManaged(t *testing.T) {
	v := api.VM{VMID: 200, Name: "unifi", Node: "pve-g3-1"}
	got := renderVMSummary(v, api.VMStatus{}, api.VMConfig{}, nil, "started", true)

	if !strings.Contains(got, "HA State") || !strings.Contains(got, "started") {
		t.Errorf("renderVMSummary() = %q, want it to report HA State as started", got)
	}
}

func TestRenderVMSummaryStoppedVMOmitsIPs(t *testing.T) {
	v := api.VM{VMID: 201, Name: "stopped-vm", Node: "pve1"}
	status := api.VMStatus{Status: "stopped", CPUs: 4}
	config := api.VMConfig{Fields: map[string]string{"agent": "0"}}

	// A stopped/agent-less VM's guest-agent call errors, so runVMSummary
	// passes through an empty/nil slice for interfaces best-effort.
	got := renderVMSummary(v, status, config, nil, "", false)

	if strings.Contains(got, "IPs:") {
		t.Errorf("renderVMSummary() = %q, want no IPs section when interfaces is empty", got)
	}
	if !strings.Contains(got, "Guest Agent") || !strings.Contains(got, "no") {
		t.Errorf("renderVMSummary() = %q, want Guest Agent: no", got)
	}
	if !strings.Contains(got, "Memory usage") || !strings.Contains(got, "N/A") {
		t.Errorf("renderVMSummary() = %q, want Memory usage: N/A when maxmem is 0", got)
	}
}
