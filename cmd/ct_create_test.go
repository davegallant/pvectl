package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

// TestPromptChoiceNoChoices doesn't need to mock stdin: with zero choices,
// promptChoice must error out before it ever tries to prompt, matching
// promptTargetNode's "reject before reading input" precedent.
func TestPromptChoiceNoChoices(t *testing.T) {
	if _, err := promptChoice("node", nil); err == nil {
		t.Error("promptChoice() error = nil, want an error when there are no choices")
	}
}

func TestPromptNodeNoNodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if _, err := promptNode(client); err == nil {
		t.Error("promptNode() error = nil, want an error when there are no cluster nodes")
	}
}

func TestPromptTemplateNoStorages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No storage entries for pve1, so storageNamesForNode returns
		// none and ListTemplates is never even called with a storage to
		// query.
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if _, err := promptTemplate(client, "pve1"); err == nil {
		t.Error("promptTemplate() error = nil, want an error when there are no templates available")
	}
}

// TestListNodeStoragesFiltersByContentType confirms the content-type
// split promptRootfsStorage relies on: a storage without "rootdir" in its
// content list (e.g. one that only serves ISOs/templates/backups) is
// identifiable as not rootdir-capable — found the hard way against a real
// cluster where "local" doesn't support container directories at all,
// while "local-lvm" does. This checks ListNodeStorages/SupportsContent
// directly rather than through promptRootfsStorage, since a scenario with
// one valid choice would make that function actually try to read stdin.
func TestListNodeStoragesFiltersByContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"storage": "local", "content": "vztmpl,backup,iso"},
				{"storage": "local-lvm", "content": "rootdir,images"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	storages, err := client.ListNodeStorages(context.Background(), "pve1")
	if err != nil {
		t.Fatalf("ListNodeStorages() error = %v", err)
	}
	var rootdirCapable []string
	for _, s := range storages {
		if s.SupportsContent("rootdir") {
			rootdirCapable = append(rootdirCapable, s.Storage)
		}
	}
	if len(rootdirCapable) != 1 || rootdirCapable[0] != "local-lvm" {
		t.Errorf("rootdir-capable storages = %v, want only [local-lvm]", rootdirCapable)
	}
}

func TestPromptRootfsStorageNoRootdirStorages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"storage": "local", "content": "vztmpl,backup,iso"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if _, err := promptRootfsStorage(client, "pve1"); err == nil {
		t.Error("promptRootfsStorage() error = nil, want an error when no storage supports rootdir")
	}
}

// TestRunCtCreateDirectPath exercises the fully-flagged path (matching
// TestRunCtMigrateDirectPath's style): every value resolved from a flag,
// so no prompt ever touches stdin, followed by a --start-triggered
// second task.
func TestRunCtCreateDirectPath(t *testing.T) {
	var createBody, startPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api2/json/nodes/pve1/lxc" && r.Method == http.MethodPost:
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

	ctCreateNode = "pve1"
	ctCreateTemplate = "local:vztmpl/ubuntu-24.04-standard_24.04-2_amd64.tar.zst"
	ctCreateStorage = "local-lvm"
	ctCreateHostname = "web01"
	ctCreateVMID = 105
	ctCreateCores = 1
	ctCreateMemory = 512
	ctCreateSwap = 512
	ctCreateDiskSize = 8
	ctCreateNet0 = "name=eth0,bridge=vmbr0,ip=dhcp"
	ctCreateUnprivileged = true
	ctCreateArch = "amd64"
	ctCreateStart = true
	defer func() {
		ctCreateNode = ""
		ctCreateTemplate = ""
		ctCreateStorage = ""
		ctCreateHostname = ""
		ctCreateVMID = 0
		ctCreateStart = false
	}()

	if err := runCtCreate(client, true); err != nil {
		t.Fatalf("runCtCreate() error = %v", err)
	}
	if !strings.Contains(createBody, "hostname=web01") {
		t.Errorf("create body = %q, want it to contain hostname=web01", createBody)
	}
	if !strings.Contains(createBody, "rootfs=local-lvm%3A8") {
		t.Errorf("create body = %q, want it to contain rootfs=local-lvm:8 (url-encoded)", createBody)
	}
	if startPath != "/api2/json/nodes/pve1/lxc/105/status/start" {
		t.Errorf("start path = %q, want /api2/json/nodes/pve1/lxc/105/status/start", startPath)
	}
}

// TestRunCtCreateSkipsStartWhenNotRequested confirms --start (or its
// prompt) isn't the only thing gating a second task: with startFlagSet
// true and ctCreateStart false, no start request should ever fire.
func TestRunCtCreateSkipsStartWhenNotRequested(t *testing.T) {
	startCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api2/json/nodes/pve1/lxc" && r.Method == http.MethodPost:
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

	ctCreateNode = "pve1"
	ctCreateTemplate = "local:vztmpl/ubuntu-24.04-standard_24.04-2_amd64.tar.zst"
	ctCreateStorage = "local-lvm"
	ctCreateHostname = "web01"
	ctCreateVMID = 105
	ctCreateDiskSize = 8
	ctCreateStart = false
	defer func() {
		ctCreateNode = ""
		ctCreateTemplate = ""
		ctCreateStorage = ""
		ctCreateHostname = ""
		ctCreateVMID = 0
	}()

	if err := runCtCreate(client, true); err != nil {
		t.Fatalf("runCtCreate() error = %v", err)
	}
	if startCalled {
		t.Error("runCtCreate() called start despite --start not being set")
	}
}

func TestCreateCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"ct", "create"}); err != nil {
		t.Errorf("rootCmd.Find([ct create]) error = %v", err)
	}
}
