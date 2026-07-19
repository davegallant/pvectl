package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestRenderSnapshots(t *testing.T) {
	snapshots := []api.Snapshot{
		{Name: "before-update", Description: "pre-update", SnapTime: 1000},
		{Name: "after-update", Description: "post-update", SnapTime: 2000, Parent: "before-update"},
	}

	got := renderSnapshots(snapshots)

	for _, want := range []string{"NAME", "DESCRIPTION", "DATE", "before-update", "after-update", "pre-update", "post-update"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderSnapshots() = %q, want it to contain %q", got, want)
		}
	}
}

func TestRenderSnapshotsCollapsesMultilineDescription(t *testing.T) {
	snapshots := []api.Snapshot{
		{Name: "snap1", Description: "line one\nline two", SnapTime: 1000},
	}

	got := renderSnapshots(snapshots)

	if strings.Contains(got, "\nline two") {
		t.Errorf("renderSnapshots() = %q, want the embedded newline collapsed so it doesn't break the table row", got)
	}
	if !strings.Contains(got, "line one line two") {
		t.Errorf("renderSnapshots() = %q, want the description joined with a space", got)
	}
}

func TestRunListSnapshots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve1/lxc/101/snapshot" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"name": "snap1", "description": "test", "snaptime": 1000},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if err := runListSnapshots(client, "pve1", 101, "web"); err != nil {
		t.Fatalf("runListSnapshots() error = %v", err)
	}
}

func TestRunListSnapshotsVM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve1/qemu/201/snapshot" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if err := runListSnapshotsVM(client, "pve1", 201, "web"); err != nil {
		t.Fatalf("runListSnapshotsVM() error = %v", err)
	}
}

func TestRunRollbackSnapshotNoSnapshots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	// No snapshots means the empty-listing short-circuit fires before any
	// stdin read (the interactive name prompt / "type yes" confirmation),
	// so this is safe to run without a stdin fixture — same reasoning as
	// TestDispatchActionSnapshots below.
	if err := runRollbackSnapshot(client, "pve1", 101, "web", "", false); err != nil {
		t.Fatalf("runRollbackSnapshot() error = %v", err)
	}
}

func TestRunRollbackSnapshotVMNoSnapshots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if err := runRollbackSnapshotVM(client, "pve1", 201, "web", "", false); err != nil {
		t.Fatalf("runRollbackSnapshotVM() error = %v", err)
	}
}

func TestRunRollbackSnapshotUnknownNameRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"name": "snap1", "snaptime": 1000}},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	// --snapshot-name is set to a name that never appeared in the listing, so this
	// must be rejected before ever reaching the rollback API (and, since
	// skipConfirm's true, without any stdin read either).
	err := runRollbackSnapshot(client, "pve1", 101, "web", "typo-name", true)
	if err == nil {
		t.Fatal("runRollbackSnapshot() error = nil, want an error for an unknown snapshot name")
	}
	if !strings.Contains(err.Error(), "no snapshot named") {
		t.Errorf("runRollbackSnapshot() error = %q, want it to mention the unknown snapshot name", err.Error())
	}
}

func TestSnapshotsCommandsRegistered(t *testing.T) {
	for _, args := range [][]string{
		{"ct", "snapshots", "create"}, {"ct", "snapshots", "list"}, {"ct", "snapshots", "delete"}, {"ct", "snapshots", "rollback"},
		{"qm", "snapshots", "create"}, {"qm", "snapshots", "list"}, {"qm", "snapshots", "delete"}, {"qm", "snapshots", "rollback"},
	} {
		found, _, err := rootCmd.Find(args)
		if err != nil {
			t.Errorf("rootCmd.Find(%v) error = %v", args, err)
			continue
		}
		want := args[len(args)-1]
		if found.Name() != want {
			t.Errorf("Find(%v).Name() = %q, want %q", args, found.Name(), want)
		}
	}
}

func TestBackupsCommandsRegistered(t *testing.T) {
	for _, args := range [][]string{
		{"ct", "backups", "create"}, {"ct", "backups", "list"}, {"ct", "backups", "delete"},
		{"qm", "backups", "create"}, {"qm", "backups", "list"}, {"qm", "backups", "delete"},
	} {
		found, _, err := rootCmd.Find(args)
		if err != nil {
			t.Errorf("rootCmd.Find(%v) error = %v", args, err)
			continue
		}
		want := args[len(args)-1]
		if found.Name() != want {
			t.Errorf("Find(%v).Name() = %q, want %q", args, found.Name(), want)
		}
	}
}

// TestNoTopLevelSnapshotOrBackupCommand documents the consistency fix:
// creation lives only under the plural group (`ct/qm snapshots create`,
// `ct/qm backups create`) — there's deliberately no bare top-level
// `ct/qm snapshot`/`ct/qm backup` command anymore, matching every other
// multi-verb resource's pattern (delete/list/rollback already nested
// under the plural group).
func TestNoTopLevelSnapshotOrBackupCommand(t *testing.T) {
	for _, args := range [][]string{
		{"ct", "snapshot"}, {"ct", "backup"},
		{"qm", "snapshot"}, {"qm", "backup"},
	} {
		found, _, err := rootCmd.Find(args)
		if err == nil && found.Name() == args[len(args)-1] {
			t.Errorf("rootCmd.Find(%v) unexpectedly found a top-level command; want it to only exist under the plural group as \"create\"", args)
		}
	}
}

func TestRunSnapshotsActionNoSnapshots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	c := api.Container{VMID: 101, Name: "web", Node: "pve1"}

	if err := runSnapshotsAction(client, c); err != nil {
		t.Errorf("runSnapshotsAction() error = %v", err)
	}
}

func TestRunSnapshotsVMActionNoSnapshots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	v := api.VM{VMID: 201, Name: "web", Node: "pve1"}

	if err := runSnapshotsVMAction(client, v); err != nil {
		t.Errorf("runSnapshotsVMAction() error = %v", err)
	}
}

func TestRunRollbackSnapshotActionNoSnapshots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	c := api.Container{VMID: 101, Name: "web", Node: "pve1"}

	if err := runRollbackSnapshotAction(client, c); err != nil {
		t.Errorf("runRollbackSnapshotAction() error = %v", err)
	}
}

func TestRunRollbackSnapshotVMActionNoSnapshots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	v := api.VM{VMID: 201, Name: "web", Node: "pve1"}

	if err := runRollbackSnapshotVMAction(client, v); err != nil {
		t.Errorf("runRollbackSnapshotVMAction() error = %v", err)
	}
}
