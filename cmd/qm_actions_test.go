package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestRunStartVM(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// runProgressAction polls TaskStatus to completion even when stdout
		// isn't a terminal; answer the poll with a finished task and only
		// let the trigger POST set gotPath.
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

	if err := runStartVM(client, v); err != nil {
		t.Fatalf("runStartVM() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/qemu/201/status/start" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestRunStopVM(t *testing.T) {
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
	v := api.VM{VMID: 201, Name: "web", Node: "pve1"}

	if err := runStopVM(client, v); err != nil {
		t.Fatalf("runStopVM() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/qemu/201/status/stop" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestRunShutdownVM(t *testing.T) {
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
	v := api.VM{VMID: 201, Name: "web", Node: "pve1"}

	if err := runShutdownVM(client, v); err != nil {
		t.Fatalf("runShutdownVM() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/qemu/201/status/shutdown" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestRunRebootVM(t *testing.T) {
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
	v := api.VM{VMID: 201, Name: "web", Node: "pve1"}

	if err := runRebootVM(client, v); err != nil {
		t.Fatalf("runRebootVM() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/qemu/201/status/reboot" {
		t.Errorf("path = %q", gotPath)
	}
}

// TestRunTemplateVMSkipConfirm mirrors TestRunTemplateSkipConfirm for
// QEMU VMs.
func TestRunTemplateVMSkipConfirm(t *testing.T) {
	var gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	v := api.VM{VMID: 201, Name: "vm01", Node: "pve1"}

	qmTemplateYes = true
	defer func() { qmTemplateYes = false }()
	if err := runTemplateVM(client, v); err != nil {
		t.Fatalf("runTemplateVM() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api2/json/nodes/pve1/qemu/201/template" {
		t.Errorf("path = %q, want .../qemu/201/template", gotPath)
	}
}

func TestTemplateVMCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"qm", "template"}); err != nil {
		t.Errorf("rootCmd.Find([qm template]) error = %v", err)
	}
}

func TestSimpleVMActionCommandsRegistered(t *testing.T) {
	for _, name := range []string{"start", "stop", "reboot", "unlock"} {
		found, _, err := rootCmd.Find([]string{"qm", name})
		if err != nil {
			t.Errorf("rootCmd.Find(%q) error = %v", name, err)
			continue
		}
		if found.Name() != name {
			t.Errorf("Find(%q).Name() = %q, want %q", name, found.Name(), name)
		}
	}
}
