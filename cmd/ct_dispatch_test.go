package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestCtCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"ct"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("ct") error = %v`, err)
	}
	if found.Use != "ct" {
		t.Errorf(`Find("ct").Use = %q, want "ct"`, found.Use)
	}
}

func TestCtListCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"ct", "list"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("ct", "list") error = %v`, err)
	}
	if found.Name() != "list" {
		t.Errorf(`Find("ct", "list").Name() = %q, want "list"`, found.Name())
	}
}

func TestRenderContainerList(t *testing.T) {
	containers := []api.Container{
		{VMID: 101, Name: "web01", Node: "pve1", Status: "running"},
		{VMID: 102, Name: "db01", Node: "pve2", Status: "stopped"},
	}

	got := renderContainerList(containers)

	for _, want := range []string{"VMID", "NAME", "NODE", "STATUS", "101", "web01", "pve1", "running", "102", "db01", "pve2", "stopped"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderContainerList() = %q, want it to contain %q", got, want)
		}
	}
}

func TestRunCtList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"type": "lxc", "vmid": 101, "name": "web01", "node": "pve1", "status": "running"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if err := runCtList(client); err != nil {
		t.Fatalf("runCtList() error = %v", err)
	}
}
