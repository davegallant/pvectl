package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientListSnapshotsFiltersCurrentAndSortsNewestFirst(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve1/lxc/101/snapshot" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"name": "before-update", "description": "pre-update", "snaptime": 1000},
				{"name": "current", "description": "You are here!"},
				{"name": "after-update", "description": "post-update", "snaptime": 2000, "parent": "before-update"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	snapshots, err := client.ListSnapshots(context.Background(), "pve1", 101)
	if err != nil {
		t.Fatalf("ListSnapshots() error = %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("len(snapshots) = %d, want 2 (the synthetic \"current\" entry excluded)", len(snapshots))
	}
	if snapshots[0].Name != "after-update" || snapshots[1].Name != "before-update" {
		t.Errorf("snapshots = %+v, want after-update then before-update (newest first)", snapshots)
	}
	if snapshots[0].Parent != "before-update" {
		t.Errorf("snapshots[0].Parent = %q, want %q", snapshots[0].Parent, "before-update")
	}
}

func TestClientListSnapshotsVM(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve1/qemu/201/snapshot" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"name": "current"},
				{"name": "snap1", "description": "test", "snaptime": 1500},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	snapshots, err := client.ListSnapshotsVM(context.Background(), "pve1", 201)
	if err != nil {
		t.Fatalf("ListSnapshotsVM() error = %v", err)
	}
	if len(snapshots) != 1 || snapshots[0].Name != "snap1" {
		t.Errorf("ListSnapshotsVM() = %+v, want a single snap1 entry", snapshots)
	}
}

func TestClientDeleteSnapshot(t *testing.T) {
	var gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:..."})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	upid, err := client.DeleteSnapshot(context.Background(), "pve1", 101, "before-update")
	if err != nil {
		t.Fatalf("DeleteSnapshot() error = %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/api2/json/nodes/pve1/lxc/101/snapshot/before-update" {
		t.Errorf("path = %q", gotPath)
	}
	if upid != "UPID:pve1:..." {
		t.Errorf("upid = %q, want the UPID from the response", upid)
	}
}

func TestClientDeleteSnapshotVM(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:..."})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	if _, err := client.DeleteSnapshotVM(context.Background(), "pve1", 201, "snap1"); err != nil {
		t.Fatalf("DeleteSnapshotVM() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/qemu/201/snapshot/snap1" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestClientRollback(t *testing.T) {
	var gotPath, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:..."})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	upid, err := client.Rollback(context.Background(), "pve1", 101, "before-update")
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api2/json/nodes/pve1/lxc/101/snapshot/before-update/rollback" {
		t.Errorf("path = %q", gotPath)
	}
	if upid != "UPID:pve1:..." {
		t.Errorf("upid = %q, want the UPID from the response", upid)
	}
}

func TestClientRollbackVM(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:..."})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	if _, err := client.RollbackVM(context.Background(), "pve1", 201, "snap1"); err != nil {
		t.Fatalf("RollbackVM() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/qemu/201/snapshot/snap1/rollback" {
		t.Errorf("path = %q", gotPath)
	}
}
