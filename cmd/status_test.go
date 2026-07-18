package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want string
	}{
		{"zero", 0, "0B"},
		{"under 1K", 1023, "1023B"},
		{"exactly 1K", 1024, "1K"},
		{"fractional K", 1536, "1.5K"},
		{"exactly 1M", 1048576, "1M"},
		{"exactly 1G", 1073741824, "1G"},
		{"120G", 128849018880, "120G"},
		{"exactly 1T", 1099511627776, "1T"},
		{"8T", 8796093022208, "8T"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatBytes(tt.n); got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestRenderStatusClusteredHeader(t *testing.T) {
	status := api.ClusterStatus{Name: "homelab", Quorate: true, Nodes: map[string]api.NodeStatus{}}
	resources := api.ClusterResources{}

	got := renderStatus("8.1.4", status, resources)

	want := "Proxmox VE 8.1.4 — cluster: homelab (quorate)\n"
	if !strings.HasPrefix(got, want) {
		t.Errorf("renderStatus() = %q, want it to start with %q", got, want)
	}
}

func TestRenderStatusNotQuorateHeader(t *testing.T) {
	status := api.ClusterStatus{Name: "homelab", Quorate: false, Nodes: map[string]api.NodeStatus{}}
	resources := api.ClusterResources{}

	got := renderStatus("8.1.4", status, resources)

	if !strings.Contains(got, "(not quorate)") {
		t.Errorf("renderStatus() = %q, want it to mention \"(not quorate)\"", got)
	}
}

func TestRenderStatusStandaloneHeader(t *testing.T) {
	status := api.ClusterStatus{Standalone: true, Nodes: map[string]api.NodeStatus{}}
	resources := api.ClusterResources{}

	got := renderStatus("8.1.4", status, resources)

	want := "Proxmox VE 8.1.4 — standalone\n"
	if !strings.HasPrefix(got, want) {
		t.Errorf("renderStatus() = %q, want it to start with %q", got, want)
	}
}

func TestRenderStatusStandaloneOmitsQuorumLine(t *testing.T) {
	status := api.ClusterStatus{Standalone: true, Nodes: map[string]api.NodeStatus{
		"pve1": {IP: "10.0.0.10", Online: true},
	}}
	resources := api.ClusterResources{
		Nodes: []api.NodeResource{{Name: "pve1", Status: "online"}},
		Storage: []api.StorageResource{
			{Name: "local", Node: "pve1", Disk: 10, MaxDisk: 100, Health: "available"},
		},
	}

	got := renderStatus("8.1.4", status, resources)

	if strings.Contains(got, "Quorum") {
		t.Errorf("renderStatus() = %q, want no \"Quorum\" check line for a standalone host", got)
	}
	if !strings.Contains(got, "Cluster is healthy") {
		t.Errorf("renderStatus() = %q, want a healthy verdict", got)
	}
}

func TestRenderStatusHealthyCluster(t *testing.T) {
	status := api.ClusterStatus{Name: "homelab", Quorate: true, Nodes: map[string]api.NodeStatus{
		"pve1": {IP: "10.0.0.10", Online: true},
		"pve2": {IP: "10.0.0.11", Online: true},
	}}
	resources := api.ClusterResources{
		Nodes: []api.NodeResource{
			{Name: "pve1", Status: "online"},
			{Name: "pve2", Status: "online"},
		},
		Containers: api.ResourceCounts{Running: 41, Stopped: 1, Total: 42},
		VMs:        api.ResourceCounts{Running: 4, Stopped: 1, Total: 5},
		Storage: []api.StorageResource{
			{Name: "local", Node: "pve1", Disk: 10, MaxDisk: 100, Health: "available"},
			{Name: "local", Node: "pve2", Disk: 20, MaxDisk: 100, Health: "available"},
		},
	}

	got := renderStatus("9.2.4", status, resources)

	for _, want := range []string{
		"✓ Quorum", "quorate",
		"✓ Nodes", "2/2 online",
		"• CTs", "41 running, 1 stopped",
		"• VMs", "4 running, 1 stopped",
		"✓ Storage", "2/2 available",
		"Cluster is healthy",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderStatus() = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "need attention") {
		t.Errorf("renderStatus() = %q, want no \"need attention\" verdict for a healthy cluster", got)
	}
}

func TestRenderStatusNotQuorateFlaggedAsIssue(t *testing.T) {
	status := api.ClusterStatus{Name: "homelab", Quorate: false, Nodes: map[string]api.NodeStatus{
		"pve1": {IP: "10.0.0.10", Online: true},
	}}
	resources := api.ClusterResources{
		Nodes: []api.NodeResource{{Name: "pve1", Status: "online"}},
		Storage: []api.StorageResource{
			{Name: "local", Node: "pve1", Disk: 10, MaxDisk: 100, Health: "available"},
		},
	}

	got := renderStatus("9.2.4", status, resources)

	if !strings.Contains(got, "✗ Quorum") || !strings.Contains(got, "not quorate") {
		t.Errorf("renderStatus() = %q, want a ✗ Quorum \"not quorate\" check line", got)
	}
	if !strings.Contains(got, "need attention") {
		t.Errorf("renderStatus() = %q, want a \"need attention\" verdict when quorum is lost", got)
	}
	if strings.Contains(got, "Cluster is healthy") {
		t.Errorf("renderStatus() = %q, want no healthy verdict when quorum is lost", got)
	}
}

func TestRenderStatusOfflineNodeFlaggedAsIssue(t *testing.T) {
	status := api.ClusterStatus{Name: "homelab", Quorate: true, Nodes: map[string]api.NodeStatus{
		"pve1": {IP: "10.0.0.10", Online: true},
		"pve2": {IP: "10.0.0.11", Online: false},
	}}
	resources := api.ClusterResources{
		Nodes: []api.NodeResource{
			{Name: "pve1", Status: "online"},
			{Name: "pve2", Status: "offline"},
		},
		Storage: []api.StorageResource{
			{Name: "local", Node: "pve1", Disk: 10, MaxDisk: 100, Health: "available"},
		},
	}

	got := renderStatus("9.2.4", status, resources)

	if !strings.Contains(got, "✗ Nodes") || !strings.Contains(got, "1/2 online") {
		t.Errorf("renderStatus() = %q, want a ✗ Nodes line showing \"1/2 online\"", got)
	}
	if !strings.Contains(got, "need attention") {
		t.Errorf("renderStatus() = %q, want a \"need attention\" verdict with an offline node", got)
	}
}

func TestRenderStatusStoppedGuestsAreNotIssues(t *testing.T) {
	status := api.ClusterStatus{Name: "homelab", Quorate: true, Nodes: map[string]api.NodeStatus{
		"pve1": {IP: "10.0.0.10", Online: true},
	}}
	resources := api.ClusterResources{
		Nodes:      []api.NodeResource{{Name: "pve1", Status: "online"}},
		Containers: api.ResourceCounts{Running: 2, Stopped: 5, Total: 7},
		VMs:        api.ResourceCounts{Running: 1, Stopped: 3, Total: 4},
		Storage: []api.StorageResource{
			{Name: "local", Node: "pve1", Disk: 10, MaxDisk: 100, Health: "available"},
		},
	}

	got := renderStatus("9.2.4", status, resources)

	if !strings.Contains(got, "2 running, 5 stopped") || !strings.Contains(got, "1 running, 3 stopped") {
		t.Errorf("renderStatus() = %q, want CT/VM counts reported as informational lines", got)
	}
	if !strings.Contains(got, "Cluster is healthy") {
		t.Errorf("renderStatus() = %q, want a healthy verdict — stopped guests are not a fault", got)
	}
}

func TestRenderStatusStorageUnavailableFlaggedAsIssue(t *testing.T) {
	status := api.ClusterStatus{Name: "homelab", Quorate: true, Nodes: map[string]api.NodeStatus{
		"pve1": {IP: "10.0.0.10", Online: true},
	}}
	resources := api.ClusterResources{
		Nodes: []api.NodeResource{{Name: "pve1", Status: "online"}},
		Storage: []api.StorageResource{
			{Name: "local", Node: "pve1", Disk: 10, MaxDisk: 100, Health: "available"},
			{Name: "nfs-tank", Node: "pve1", Disk: 10, MaxDisk: 100, Health: "unavailable"},
		},
	}

	got := renderStatus("9.2.4", status, resources)

	if !strings.Contains(got, "✗ Storage") || !strings.Contains(got, "1/2 available") {
		t.Errorf("renderStatus() = %q, want a ✗ Storage line showing \"1/2 available\"", got)
	}
	if !strings.Contains(got, "need attention") {
		t.Errorf("renderStatus() = %q, want a \"need attention\" verdict with unavailable storage", got)
	}
}

func TestRenderStatusStorageNearFullWarn(t *testing.T) {
	status := api.ClusterStatus{Name: "homelab", Quorate: true, Nodes: map[string]api.NodeStatus{
		"pve1": {IP: "10.0.0.10", Online: true},
	}}
	resources := api.ClusterResources{
		Nodes: []api.NodeResource{{Name: "pve1", Status: "online"}},
		Storage: []api.StorageResource{
			{Name: "local", Node: "pve1", Disk: 10, MaxDisk: 100, Health: "available"},
			{Name: "local-lvm", Node: "pve1", Disk: 85, MaxDisk: 100, Health: "available"},
		},
	}

	got := renderStatus("9.2.4", status, resources)

	if !strings.Contains(got, "⚠ Storage") || !strings.Contains(got, "at 85%") {
		t.Errorf("renderStatus() = %q, want a ⚠ Storage line flagging local-lvm at 85%%", got)
	}
	if !strings.Contains(got, "need attention") {
		t.Errorf("renderStatus() = %q, want a \"need attention\" verdict for near-full storage", got)
	}
}

func TestRenderStatusStorageCriticallyFull(t *testing.T) {
	status := api.ClusterStatus{Name: "homelab", Quorate: true, Nodes: map[string]api.NodeStatus{
		"pve1": {IP: "10.0.0.10", Online: true},
	}}
	resources := api.ClusterResources{
		Nodes: []api.NodeResource{{Name: "pve1", Status: "online"}},
		Storage: []api.StorageResource{
			{Name: "local", Node: "pve1", Disk: 10, MaxDisk: 100, Health: "available"},
			{Name: "local-lvm", Node: "pve1", Disk: 95, MaxDisk: 100, Health: "available"},
		},
	}

	got := renderStatus("9.2.4", status, resources)

	if !strings.Contains(got, "✗ Storage") || !strings.Contains(got, "at 95%") {
		t.Errorf("renderStatus() = %q, want a ✗ Storage line flagging local-lvm at 95%%", got)
	}
	if !strings.Contains(got, "need attention") {
		t.Errorf("renderStatus() = %q, want a \"need attention\" verdict for critically full storage", got)
	}
}

func TestStorageHealth(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"available", "OK"},
		{"", "UNKNOWN"},
		{"unavailable", "UNAVAILABLE"},
	}
	for _, tt := range tests {
		if got := storageHealth(tt.status); got != tt.want {
			t.Errorf("storageHealth(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestStatusCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"status"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("status") error = %v`, err)
	}
	if found.Use != "status" {
		t.Errorf(`Find("status").Use = %q, want "status"`, found.Use)
	}
}

func TestRunStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"version": "8.1.4"}})
		case "/api2/json/cluster/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"type": "cluster", "name": "homelab", "quorate": 1},
					{"type": "node", "name": "pve1", "ip": "10.0.0.10", "online": 1},
				},
			})
		case "/api2/json/cluster/resources":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"type": "node", "node": "pve1", "status": "online", "cpu": 0.12, "maxcpu": 4, "mem": 4831838208, "maxmem": 10737418240},
					{"type": "lxc", "vmid": 101, "name": "web", "node": "pve1", "status": "running"},
				},
			})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)

	if err := runStatus(client); err != nil {
		t.Fatalf("runStatus() error = %v", err)
	}
}
