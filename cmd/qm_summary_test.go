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
		"eth0: 192.168.1.50",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVMSummary() = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "fe80::") {
		t.Errorf("renderVMSummary() = %q, want link-local IPv6 filtered out", got)
	}
}

// TestRenderVMSummaryManyVirtualInterfacesOmitsLinkLocal covers a guest
// like Home Assistant OS: many Docker/Supervisor bridge and veth
// interfaces, each contributing its own link-local IPv6 address. Only the
// routable addresses, grouped by interface, should make it into the
// output — not a wall of fe80:: noise, one line per virtual NIC.
func TestRenderVMSummaryManyVirtualInterfacesOmitsLinkLocal(t *testing.T) {
	v := api.VM{VMID: 140, Name: "homeassistant", Node: "pve-g3-1"}
	interfaces := []api.QemuInterface{
		{Name: "eth0", IPAddresses: []string{"192.168.1.44", "fe80::5fca:b809:686e:4dfe"}},
		{Name: "hassio", IPAddresses: []string{"172.30.232.1", "fd8b:837b:716c::1", "fe80::30ac:e8ff:feee:21ca"}},
		{Name: "docker0", IPAddresses: []string{"172.30.32.1", "fd0c:ac1e:2100::1", "fe80::3c9c:bdff:fe76:8865"}},
		{Name: "veth1234", IPAddresses: []string{"fe80::8b0:7fff:feb8:57cc"}},
	}

	got := renderVMSummary(v, api.VMStatus{}, api.VMConfig{}, interfaces, "", false)

	for _, want := range []string{
		"eth0: 192.168.1.44",
		"hassio: 172.30.232.1, fd8b:837b:716c::1",
		"docker0: 172.30.32.1, fd0c:ac1e:2100::1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVMSummary() = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "fe80::") {
		t.Errorf("renderVMSummary() = %q, want all link-local IPv6 filtered out", got)
	}
	if strings.Contains(got, "veth1234") {
		t.Errorf("renderVMSummary() = %q, want veth1234 omitted (only had a link-local address)", got)
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
