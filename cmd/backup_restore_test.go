package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/spf13/cobra"
)

func TestFilterBackupsByGuestType(t *testing.T) {
	backups := []api.Backup{
		{VolID: "local:backup/vzdump-lxc-101-2024_01_01.tar.zst", VMID: 101},
		{VolID: "local:backup/vzdump-qemu-201-2024_01_01.vma.zst", VMID: 201},
		{VolID: "nfs:backup/vzdump-lxc-102-2024_01_02.tar.zst", VMID: 102},
	}

	lxc := filterBackupsByGuestType(backups, "lxc")
	if len(lxc) != 2 {
		t.Fatalf("filterBackupsByGuestType(lxc) = %d backups, want 2: %+v", len(lxc), lxc)
	}
	for _, b := range lxc {
		if !strings.Contains(b.VolID, "vzdump-lxc-") {
			t.Errorf("filterBackupsByGuestType(lxc) included %q", b.VolID)
		}
	}

	qemu := filterBackupsByGuestType(backups, "qemu")
	if len(qemu) != 1 || qemu[0].VMID != 201 {
		t.Errorf("filterBackupsByGuestType(qemu) = %+v, want just the vmid-201 entry", qemu)
	}
}

func TestRenderBackupsIncludesVMID(t *testing.T) {
	out := renderBackups([]api.Backup{{VolID: "local:backup/vzdump-lxc-101-x.tar.zst", VMID: 101, Storage: "local", CTime: 1704067200}})
	if !strings.Contains(out, "VMID") {
		t.Errorf("renderBackups() header = %q, want a VMID column", out)
	}
	if !strings.Contains(out, "101") {
		t.Errorf("renderBackups() body = %q, want the backup's vmid (101)", out)
	}
}

// TestRunRestoreBackupDirectPath exercises the in-place restore path end
// to end (matching TestRunCtMigrateDirectPath's style): volid and storage
// both given directly (no prompt), skipConfirm=true (no "type yes"
// prompt), so it never touches stdin. Confirms force=1 is always sent
// (the target vmid already exists by definition in this mode).
func TestRunRestoreBackupDirectPath(t *testing.T) {
	var restoreBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api2/json/cluster/resources":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"type": "storage", "node": "pve1", "storage": "local"},
				},
			})
		case r.URL.Path == "/api2/json/nodes/pve1/storage/local/content":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"volid": "local:backup/vzdump-lxc-101-a.tar.zst", "content": "backup", "vmid": 101, "ctime": 1000, "size": 2},
				},
			})
		case r.URL.Path == "/api2/json/nodes/pve1/lxc" && r.Method == http.MethodPost:
			b, _ := io.ReadAll(r.Body)
			restoreBody = string(b)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:restore"})
		case strings.Contains(r.URL.Path, "/tasks/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	err := runRestoreBackup(client, "pve1", 101, "web01", "container",
		"local:backup/vzdump-lxc-101-a.tar.zst", "local-lvm", true, client.RestoreContainer)
	if err != nil {
		t.Fatalf("runRestoreBackup() error = %v", err)
	}

	values, err := url.ParseQuery(restoreBody)
	if err != nil {
		t.Fatalf("url.ParseQuery(%q) error = %v", restoreBody, err)
	}
	if values.Get("ostemplate") != "local:backup/vzdump-lxc-101-a.tar.zst" {
		t.Errorf(`body["ostemplate"] = %q, want the backup volid`, values.Get("ostemplate"))
	}
	if values.Get("storage") != "local-lvm" {
		t.Errorf(`body["storage"] = %q, want "local-lvm"`, values.Get("storage"))
	}
	if values.Get("force") != "1" {
		t.Errorf(`body["force"] = %q, want "1" (in-place restore always overwrites)`, values.Get("force"))
	}
}

// TestRunRestoreBackupUnknownVolidRejected confirms a volid that never
// appeared in the guest's own backup listing is rejected before it can
// reach the restore API — same typo-guard discipline as
// runDeleteBackup's volid lookup.
func TestRunRestoreBackupUnknownVolidRejected(t *testing.T) {
	restoreCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/cluster/resources":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"type": "storage", "node": "pve1", "storage": "local"}},
			})
		case "/api2/json/nodes/pve1/storage/local/content":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"volid": "local:backup/vzdump-lxc-101-a.tar.zst", "content": "backup", "vmid": 101, "ctime": 1000, "size": 2},
				},
			})
		case "/api2/json/nodes/pve1/lxc":
			restoreCalled = true
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:..."})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	err := runRestoreBackup(client, "pve1", 101, "web01", "container", "local:backup/does-not-exist.tar.zst", "local-lvm", true, client.RestoreContainer)
	if err == nil {
		t.Error("runRestoreBackup() error = nil, want an error for an unknown volid")
	}
	if restoreCalled {
		t.Error("runRestoreBackup() called the restore API despite an unknown volid")
	}
}

// TestRunRestoreFromNodeNewVMIDSkipsConfirmAndForce confirms disaster
// recovery onto a genuinely free vmid needs no confirmation and sends no
// force param — matching `ct create`'s no-confirmation-for-new-resources
// behavior. skipConfirm is left false to prove the confirmation branch
// really is skipped, not merely bypassed by the flag.
func TestRunRestoreFromNodeNewVMIDSkipsConfirmAndForce(t *testing.T) {
	var restoreBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api2/json/nodes/pve1/lxc" && r.Method == http.MethodPost:
			b, _ := io.ReadAll(r.Body)
			restoreBody = string(b)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:restore"})
		case strings.Contains(r.URL.Path, "/tasks/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	backups := []api.Backup{{VolID: "local:backup/vzdump-lxc-101-a.tar.zst", VMID: 101, Storage: "local"}}
	err := runRestoreFromNode(client, "pve1", "container", backups, 0, "local:backup/vzdump-lxc-101-a.tar.zst", "local-lvm", false,
		client.RestoreContainer, func(vmid int) (bool, error) { return false, nil })
	if err != nil {
		t.Fatalf("runRestoreFromNode() error = %v", err)
	}

	values, err := url.ParseQuery(restoreBody)
	if err != nil {
		t.Fatalf("url.ParseQuery(%q) error = %v", restoreBody, err)
	}
	if values.Get("vmid") != "101" {
		t.Errorf(`body["vmid"] = %q, want the backup's own recorded vmid (101)`, values.Get("vmid"))
	}
	if values.Has("force") {
		t.Errorf("body = %q, want force omitted for a genuinely free vmid", restoreBody)
	}
}

// TestRunRestoreFromNodeExistingVMIDRequiresForce confirms disaster
// recovery onto a vmid that already exists sends force=1 (skipConfirm=
// true, so it never touches stdin for the "type yes" prompt).
func TestRunRestoreFromNodeExistingVMIDRequiresForce(t *testing.T) {
	var restoreBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api2/json/nodes/pve1/lxc" && r.Method == http.MethodPost:
			b, _ := io.ReadAll(r.Body)
			restoreBody = string(b)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:restore"})
		case strings.Contains(r.URL.Path, "/tasks/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	backups := []api.Backup{{VolID: "local:backup/vzdump-lxc-101-a.tar.zst", VMID: 101, Storage: "local"}}
	// targetVMID=105 overrides the backup's own vmid (101) — cloning under
	// a new id — and 105 is reported as already existing.
	err := runRestoreFromNode(client, "pve1", "container", backups, 105, "local:backup/vzdump-lxc-101-a.tar.zst", "local-lvm", true,
		client.RestoreContainer, func(vmid int) (bool, error) { return vmid == 105, nil })
	if err != nil {
		t.Fatalf("runRestoreFromNode() error = %v", err)
	}

	values, err := url.ParseQuery(restoreBody)
	if err != nil {
		t.Fatalf("url.ParseQuery(%q) error = %v", restoreBody, err)
	}
	if values.Get("vmid") != "105" {
		t.Errorf(`body["vmid"] = %q, want the --vmid override (105)`, values.Get("vmid"))
	}
	if values.Get("force") != "1" {
		t.Errorf(`body["force"] = %q, want "1" for an already-existing target vmid`, values.Get("force"))
	}
}

// TestRunCtBackupsRestoreRejectsArgWithNode and its qm mirror confirm the
// mode switch is a hard validation error, not a silent precedence rule —
// checked before loadClient() is ever called, so this needs no server or
// config/keychain fixture (see runCtBackupsRestore's ordering).
func TestRunCtBackupsRestoreRejectsArgWithNode(t *testing.T) {
	testCmd := &cobra.Command{}
	testCmd.Flags().StringVar(&ctRestoreNode, "node", "", "")
	if err := testCmd.Flags().Set("node", "pve1"); err != nil {
		t.Fatalf("Set(node) error = %v", err)
	}
	err := runCtBackupsRestore(testCmd, []string{"101"})
	if err == nil || !strings.Contains(err.Error(), "cannot combine") {
		t.Errorf("runCtBackupsRestore() error = %v, want a 'cannot combine' validation error", err)
	}
}

func TestRunQmBackupsRestoreRejectsArgWithNode(t *testing.T) {
	testCmd := &cobra.Command{}
	testCmd.Flags().StringVar(&qmRestoreNode, "node", "", "")
	if err := testCmd.Flags().Set("node", "pve1"); err != nil {
		t.Fatalf("Set(node) error = %v", err)
	}
	err := runQmBackupsRestore(testCmd, []string{"201"})
	if err == nil || !strings.Contains(err.Error(), "cannot combine") {
		t.Errorf("runQmBackupsRestore() error = %v, want a 'cannot combine' validation error", err)
	}
}

func TestBackupsRestoreCommandsRegistered(t *testing.T) {
	for _, args := range [][]string{{"ct", "backups", "restore"}, {"qm", "backups", "restore"}} {
		if _, _, err := rootCmd.Find(args); err != nil {
			t.Errorf("rootCmd.Find(%v) error = %v", args, err)
		}
	}
}
