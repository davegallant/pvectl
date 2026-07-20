package cmd

import (
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestRenderSummaryRunningContainer(t *testing.T) {
	c := api.Container{VMID: 104, Name: "immich", Node: "pve-g3-1", Status: "running"}
	status := api.LXCStatus{
		Status: "running", CPU: 0.0037, CPUs: 4,
		Mem: 1470000000, MaxMem: 6500000000,
		Swap: 0, MaxSwap: 0,
		Disk: 13980000000, MaxDisk: 20960000000,
	}
	config := api.Config{Fields: map[string]string{"unprivileged": "1"}}
	interfaces := []api.LXCInterface{
		{Name: "eth0", Inet: "192.168.1.24/24", Inet6: "fe80::be24:11ff:feac:5f59/64"},
	}

	got := renderSummary(c, status, config, interfaces, "", false)

	if !strings.HasPrefix(got, `Container 104 (immich) on node "pve-g3-1"`) {
		t.Errorf("renderSummary() = %q, want it to start with the container header", got)
	}
	for _, want := range []string{
		"Status", "running",
		"HA State", "none",
		"Unprivileged", "yes",
		"0.37% of 4 CPUs",
		"22.62% (1.4G of 6.1G)",
		"SWAP usage", "N/A",
		"192.168.1.24",
		"fe80::be24:11ff:feac:5f59",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderSummary() = %q, want it to contain %q", got, want)
		}
	}
}

func TestRenderSummaryHAManaged(t *testing.T) {
	c := api.Container{VMID: 104, Name: "immich", Node: "pve-g3-1"}
	got := renderSummary(c, api.LXCStatus{}, api.Config{}, nil, "started", true)

	if !strings.Contains(got, "HA State") || !strings.Contains(got, "started") {
		t.Errorf("renderSummary() = %q, want it to report HA State as started", got)
	}
}

func TestRenderSummaryStoppedContainerOmitsIPs(t *testing.T) {
	c := api.Container{VMID: 105, Name: "stopped-ct", Node: "pve1"}
	status := api.LXCStatus{Status: "stopped", CPUs: 4}
	config := api.Config{Fields: map[string]string{"unprivileged": "0"}}

	// A stopped container's /interfaces call errors, so runSummary passes
	// through an empty/nil slice for interfaces best-effort.
	got := renderSummary(c, status, config, nil, "", false)

	if strings.Contains(got, "IPs:") {
		t.Errorf("renderSummary() = %q, want no IPs section when interfaces is empty", got)
	}
	if !strings.Contains(got, "Unprivileged") || !strings.Contains(got, "no") {
		t.Errorf("renderSummary() = %q, want Unprivileged: no", got)
	}
	if !strings.Contains(got, "Memory usage") || !strings.Contains(got, "N/A") {
		t.Errorf("renderSummary() = %q, want Memory usage: N/A when maxmem is 0", got)
	}
}

func TestFormatCPUUsage(t *testing.T) {
	if got := formatCPUUsage(0.0037, 4); got != "0.37% of 4 CPUs" {
		t.Errorf("formatCPUUsage() = %q, want %q", got, "0.37% of 4 CPUs")
	}
}

func TestFormatUsage(t *testing.T) {
	tests := []struct {
		name string
		used int64
		max  int64
		want string
	}{
		{"no max", 0, 0, "N/A"},
		{"half", 512, 1024, "50.00% (512B of 1K)"},
		{"zero used", 0, 1073741824, "0.00% (0B of 1G)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatUsage(tt.used, tt.max); got != tt.want {
				t.Errorf("formatUsage(%d, %d) = %q, want %q", tt.used, tt.max, got, tt.want)
			}
		})
	}
}

func TestStripCIDR(t *testing.T) {
	tests := []struct{ in, want string }{
		{"192.168.1.24/24", "192.168.1.24"},
		{"fe80::be24:11ff:feac:5f59/64", "fe80::be24:11ff:feac:5f59"},
		{"", ""},
		{"no-cidr", "no-cidr"},
	}
	for _, tt := range tests {
		if got := stripCIDR(tt.in); got != tt.want {
			t.Errorf("stripCIDR(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
