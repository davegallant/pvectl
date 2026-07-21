package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestNodesCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"nodes"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("nodes") error = %v`, err)
	}
	if found.Use != "nodes" {
		t.Errorf(`Find("nodes").Use = %q, want "nodes"`, found.Use)
	}
}

func TestRenderNodesSortsAndMergesIP(t *testing.T) {
	status := api.ClusterStatus{
		Nodes: map[string]api.NodeStatus{
			"pve02": {IP: "10.0.0.11", Online: true},
			"pve01": {IP: "10.0.0.10", Online: true},
		},
	}
	nodes := []api.NodeResource{
		{Name: "pve02", Status: "online", CPU: 0.15, MaxCPU: 4, Mem: 17179869184, MaxMem: 68719476736},
		{Name: "pve01", Status: "online", CPU: 0.30, MaxCPU: 4, Mem: 51539607552, MaxMem: 137438953472},
	}

	got := renderNodes(status, nodes)

	for _, want := range []string{"pve01", "10.0.0.10", "30%", "pve02", "10.0.0.11", "15%"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderNodes() = %q, want it to contain %q", got, want)
		}
	}
	if strings.Index(got, "pve01") > strings.Index(got, "pve02") {
		t.Errorf("renderNodes() lists pve02 before pve01, want nodes sorted by name")
	}
}

func TestRenderNodesMissingFromClusterStatusShowsDash(t *testing.T) {
	status := api.ClusterStatus{Nodes: map[string]api.NodeStatus{}}
	nodes := []api.NodeResource{
		{Name: "pve01", Status: "online", CPU: 0, MaxCPU: 4, Mem: 0, MaxMem: 0},
	}

	got := renderNodes(status, nodes)

	var nodeLine string
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "pve01") {
			nodeLine = line
		}
	}
	if nodeLine == "" {
		t.Fatalf("renderNodes() = %q, want a line containing pve01", got)
	}
	if !strings.Contains(nodeLine, "-") {
		t.Errorf("node line = %q, want it to show \"-\" for a node missing from ClusterStatus.Nodes", nodeLine)
	}
}

func TestNodesJSONSortsAndMergesIP(t *testing.T) {
	status := api.ClusterStatus{
		Nodes: map[string]api.NodeStatus{
			"pve02": {IP: "10.0.0.11", Online: true},
		},
	}
	nodes := []api.NodeResource{
		{Name: "pve02", Status: "online", CPU: 0.15, MaxCPU: 4, Mem: 17179869184, MaxMem: 68719476736},
		{Name: "pve01", Status: "online", CPU: 0.30, MaxCPU: 4, Mem: 51539607552, MaxMem: 137438953472},
	}

	got := nodesJSON(status, nodes)

	if len(got) != 2 {
		t.Fatalf("nodesJSON() returned %d entries, want 2", len(got))
	}
	if got[0].Name != "pve01" || got[1].Name != "pve02" {
		t.Errorf("nodesJSON() = %+v, want sorted by name (pve01, pve02)", got)
	}
	if got[0].IP != "" {
		t.Errorf("pve01 IP = %q, want empty (missing from ClusterStatus.Nodes)", got[0].IP)
	}
	if got[1].IP != "10.0.0.11" {
		t.Errorf("pve02 IP = %q, want %q", got[1].IP, "10.0.0.11")
	}
}

func TestRunNodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/cluster/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"type": "cluster", "name": "homelab", "quorate": 1},
					{"type": "node", "name": "pve01", "ip": "10.0.0.10", "online": 1},
				},
			})
		case "/api2/json/cluster/resources":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"type": "node", "node": "pve01", "status": "online", "cpu": 0.3, "maxcpu": 4, "mem": 51539607552, "maxmem": 137438953472},
				},
			})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)

	if err := runNodes(client); err != nil {
		t.Fatalf("runNodes() error = %v", err)
	}
}
