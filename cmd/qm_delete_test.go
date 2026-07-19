package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

// TestRunDeleteVMSkipConfirm is TestRunDeleteContainerSkipConfirm's
// mirror for QEMU VMs — see its comment.
func TestRunDeleteVMSkipConfirm(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete:
			gotPath = r.URL.Path + "?" + r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:destroy"})
		case strings.Contains(r.URL.Path, "/tasks/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if err := runDeleteVM(client, "pve1", 201, "web01", true, true); err != nil {
		t.Fatalf("runDeleteVM() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/qemu/201?purge=1" {
		t.Errorf("delete path = %q, want /api2/json/nodes/pve1/qemu/201?purge=1", gotPath)
	}
}

func TestQmDeleteCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"qm", "delete"}); err != nil {
		t.Errorf("rootCmd.Find([qm delete]) error = %v", err)
	}
}

func TestRunDeleteActionSkipsConfirmWhenYesSet(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete:
			gotPath = r.URL.Path
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:destroy"})
		case strings.Contains(r.URL.Path, "/tasks/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	c := api.Container{VMID: 101, Name: "web", Node: "pve1"}

	// Setting ctDeleteYes directly (skipping confirmation) is the only
	// way to exercise runDeleteAction without touching stdin — unlike the
	// snapshot/backup tests, delete has no "empty listing" short-circuit
	// to fall back on.
	ctDeleteYes = true
	defer func() { ctDeleteYes = false }()

	if err := runDeleteAction(client, c); err != nil {
		t.Errorf("runDeleteAction() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/lxc/101" {
		t.Errorf("delete path = %q", gotPath)
	}
}
