package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestQmCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"qm"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("qm") error = %v`, err)
	}
	if found.Use != "qm" {
		t.Errorf(`Find("qm").Use = %q, want "qm"`, found.Use)
	}
}

func TestQmListAndSelectCommandsRegistered(t *testing.T) {
	for _, use := range []string{"list", "select"} {
		found, _, err := rootCmd.Find([]string{"qm", use})
		if err != nil {
			t.Errorf(`rootCmd.Find("qm", %q) error = %v`, use, err)
			continue
		}
		if found.Name() != use {
			t.Errorf(`Find("qm", %q).Name() = %q, want %q`, use, found.Name(), use)
		}
	}
}

func TestRenderVMList(t *testing.T) {
	vms := []api.VM{
		{VMID: 201, Name: "opnsense", Node: "pve1", Status: "running"},
		{VMID: 202, Name: "media-server", Node: "pve2", Status: "stopped"},
	}

	got := renderVMList(vms)

	for _, want := range []string{"VMID", "NAME", "NODE", "STATUS", "201", "opnsense", "pve1", "running", "202", "media-server", "pve2", "stopped"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVMList() = %q, want it to contain %q", got, want)
		}
	}
}

func TestRunQmList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"type": "qemu", "vmid": 201, "name": "opnsense", "node": "pve1", "status": "running"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if err := runQmList(client); err != nil {
		t.Fatalf("runQmList() error = %v", err)
	}
}

func TestDispatchVMActionStart(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// dispatchVMAction(start) now polls the task to completion (even
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
	v := api.VM{VMID: 201, Name: "web", Node: "pve1"}

	if err := dispatchVMAction(client, "start", v); err != nil {
		t.Fatalf("dispatchVMAction(start) error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/qemu/201/status/start" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestDispatchVMActionUnknown(t *testing.T) {
	client := api.NewClient("https://unused.invalid:8006", "user@pve!test", "secret", true)
	v := api.VM{VMID: 201, Name: "web", Node: "pve1"}

	if err := dispatchVMAction(client, "not-a-real-action", v); err != nil {
		t.Errorf("dispatchVMAction(unknown) error = %v, want nil (no-op)", err)
	}
}
