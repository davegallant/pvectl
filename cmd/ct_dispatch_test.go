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

func TestCtListAndSelectCommandsRegistered(t *testing.T) {
	for _, use := range []string{"list", "select"} {
		found, _, err := rootCmd.Find([]string{"ct", use})
		if err != nil {
			t.Errorf(`rootCmd.Find("ct", %q) error = %v`, use, err)
			continue
		}
		if found.Name() != use {
			t.Errorf(`Find("ct", %q).Name() = %q, want %q`, use, found.Name(), use)
		}
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

func TestDispatchActionStart(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// dispatchAction(start) now polls the task to completion (even
		// non-interactively), so answer the status poll and only let the
		// trigger POST set gotPath.
		if strings.Contains(r.URL.Path, "/tasks/") {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
			return
		}
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:..."})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	c := api.Container{VMID: 101, Name: "web", Node: "pve1"}

	if err := dispatchAction(client, "start", c); err != nil {
		t.Fatalf("dispatchAction(start) error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/lxc/101/status/start" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestDispatchActionUnknown(t *testing.T) {
	client := api.NewClient("https://unused.invalid:8006", "user@pve!test", "secret", true)
	c := api.Container{VMID: 101, Name: "web", Node: "pve1"}

	if err := dispatchAction(client, "not-a-real-action", c); err != nil {
		t.Errorf("dispatchAction(unknown) error = %v, want nil (no-op)", err)
	}
}
