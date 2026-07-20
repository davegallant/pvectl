package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

// TestRunQmCreateDirectPath is runCtCreate's direct-path test mirror for
// QEMU VMs: every value resolved from a flag, so no prompt ever touches
// stdin, followed by a --start-triggered second task.
func TestRunQmCreateDirectPath(t *testing.T) {
	var createBody, startPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api2/json/nodes/pve1/qemu" && r.Method == http.MethodPost:
			b, _ := io.ReadAll(r.Body)
			createBody = string(b)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:create"})
		case strings.HasSuffix(r.URL.Path, "/status/start"):
			startPath = r.URL.Path
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:start"})
		case strings.Contains(r.URL.Path, "/tasks/"):
			// runProgressAction polls both the create and start tasks to
			// completion, even non-interactively.
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)

	qmCreateNode = "pve1"
	qmCreateStorage = "local-lvm"
	qmCreateName = "web01"
	qmCreateVMID = 201
	qmCreateCores = 2
	qmCreateMemory = 2048
	qmCreateDiskSize = 32
	qmCreateNet0 = "virtio,bridge=vmbr0"
	qmCreateSCSIHW = "virtio-scsi-pci"
	qmCreateOSType = "l26"
	qmCreateISO = "local:iso/ubuntu-24.04.iso"
	qmCreateStart = true
	defer func() {
		qmCreateNode = ""
		qmCreateStorage = ""
		qmCreateName = ""
		qmCreateVMID = 0
		qmCreateISO = ""
		qmCreateStart = false
	}()

	if err := runQmCreate(client, true); err != nil {
		t.Fatalf("runQmCreate() error = %v", err)
	}
	if !strings.Contains(createBody, "name=web01") {
		t.Errorf("create body = %q, want it to contain name=web01", createBody)
	}
	if !strings.Contains(createBody, "scsi0=local-lvm%3A32") {
		t.Errorf("create body = %q, want it to contain scsi0=local-lvm:32 (url-encoded)", createBody)
	}
	if startPath != "/api2/json/nodes/pve1/qemu/201/status/start" {
		t.Errorf("start path = %q, want /api2/json/nodes/pve1/qemu/201/status/start", startPath)
	}
}

// TestRunQmCreateSkipsStartWhenNotRequested confirms --start (or its
// prompt) isn't the only thing gating a second task: with startFlagSet
// true and qmCreateStart false, no start request should ever fire.
func TestRunQmCreateSkipsStartWhenNotRequested(t *testing.T) {
	startCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api2/json/nodes/pve1/qemu" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:create"})
		case strings.HasSuffix(r.URL.Path, "/status/start"):
			startCalled = true
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:start"})
		case strings.Contains(r.URL.Path, "/tasks/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)

	qmCreateNode = "pve1"
	qmCreateStorage = "local-lvm"
	qmCreateName = "web01"
	qmCreateVMID = 201
	qmCreateDiskSize = 32
	qmCreateISO = "local:iso/ubuntu-24.04.iso"
	qmCreateStart = false
	defer func() {
		qmCreateNode = ""
		qmCreateStorage = ""
		qmCreateName = ""
		qmCreateVMID = 0
		qmCreateISO = ""
	}()

	if err := runQmCreate(client, true); err != nil {
		t.Fatalf("runQmCreate() error = %v", err)
	}
	if startCalled {
		t.Error("runQmCreate() called start despite --start not being set")
	}
}

// TestPromptISONoISOs confirms promptISO's optional-flag shape: unlike
// promptTemplate (ct_create.go), which errors out with zero choices,
// promptISO treats "nothing to attach" as a valid disk-only outcome and
// returns silently without ever touching stdin.
func TestPromptISONoISOs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No storage entries for pve1, so storageNamesForNode returns none
		// and ListISOs is never even called with a storage to query.
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	iso, err := promptISO(client, "pve1")
	if err != nil {
		t.Fatalf("promptISO() error = %v, want nil when no ISOs are available", err)
	}
	if iso != "" {
		t.Errorf("promptISO() = %q, want empty string when no ISOs are available", iso)
	}
}

func TestQmCreateCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"qm", "create"}); err != nil {
		t.Errorf("rootCmd.Find([qm create]) error = %v", err)
	}
}
