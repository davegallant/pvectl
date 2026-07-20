package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestRunStart(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// runProgressAction now polls TaskStatus to completion even when
		// stdout isn't a terminal, so answer the task-status poll with a
		// finished task and only let the trigger POST set gotPath.
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

	if err := runStart(client, c); err != nil {
		t.Fatalf("runStart() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/lxc/101/status/start" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestRunStop(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	if err := runStop(client, c); err != nil {
		t.Fatalf("runStop() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/lxc/101/status/stop" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestRunShutdown(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	if err := runShutdown(client, c); err != nil {
		t.Fatalf("runShutdown() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/lxc/101/status/shutdown" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestRunReboot(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	if err := runReboot(client, c); err != nil {
		t.Fatalf("runReboot() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/lxc/101/status/reboot" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestSimpleActionCommandsRegistered(t *testing.T) {
	for _, name := range []string{"start", "stop", "reboot"} {
		found, _, err := rootCmd.Find([]string{"ct", name})
		if err != nil {
			t.Errorf("rootCmd.Find(%q) error = %v", name, err)
			continue
		}
		if found.Name() != name {
			t.Errorf("Find(%q).Name() = %q, want %q", name, found.Name(), name)
		}
	}
}
