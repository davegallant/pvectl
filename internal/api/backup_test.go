package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

func TestBackupTriggersVzdump(t *testing.T) {
	var gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:..."})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	upid, err := client.Backup(context.Background(), "pve1", 101, "local")
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	if upid != "UPID:pve1:..." {
		t.Errorf("Backup() upid = %q", upid)
	}
	if gotPath != "/api2/json/nodes/pve1/vzdump" {
		t.Errorf("Backup() path = %q", gotPath)
	}
	// vmid is always sent; storage only when non-empty. url.Values.Encode
	// sorts keys, so the body is "storage=local&vmid=101".
	if gotBody != "storage=local&vmid=101" {
		t.Errorf("Backup() body = %q, want vmid+storage form-encoded", gotBody)
	}
}

func TestBackupOmitsEmptyStorage(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:..."})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	if _, err := client.Backup(context.Background(), "pve1", 101, ""); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	if gotBody != "vmid=101" {
		t.Errorf("Backup() body = %q, want only vmid when storage is empty", gotBody)
	}
}

func TestDeleteBackupPathEscapesVolid(t *testing.T) {
	var gotEscaped string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// r.URL.Path is the *decoded* path (the escaped "/" decodes back
		// to "/"), so to confirm the volid was sent as one opaque segment we
		// check EscapedPath, which preserves the %2F the client emitted.
		gotEscaped = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	volid := "local:backup/vzdump-lxc-101-2024_01_02.tar.zst"
	if err := client.DeleteBackup(context.Background(), "pve1", "local", volid); err != nil {
		t.Fatalf("DeleteBackup() error = %v", err)
	}
	// The volid contains a "/" (and a ":") — PathEscape percent-encodes
	// both so it survives as one opaque path segment rather than being
	// split across multiple segments.
	want := "/api2/json/nodes/pve1/storage/local/content/" + url.PathEscape(volid)
	if gotEscaped != want {
		t.Errorf("DeleteBackup() escaped path = %q, want %q (volid percent-escaped)", gotEscaped, want)
	}
}

// TestListBackupsFansOutConcurrently fans the per-storage GETs across
// goroutines and asserts they were all issued (paths touched), the
// content/vmid filters are applied, and the results are merged + sorted
// newest-first. Run under -race to lock in the concurrent-merge's
// data-race-freedom (each goroutine writes its own results[i] slot; Wait
// happens-before the read).
func TestListBackupsFansOutConcurrently(t *testing.T) {
	const node = "pve1"
	storages := []string{"local", "nfs-bulk", "pbs"}
	// Concurrency here is genuine: ListBackups fans the per-storage GETs
	// out across goroutines, so the httptest handler is entered
	// concurrently and touches a shared map — protect it with a mutex so
	// the test itself stays race-free under -race.
	var mu sync.Mutex
	pathsSeen := make(map[string]bool)
	mark := func(path string) {
		mu.Lock()
		pathsSeen[path] = true
		mu.Unlock()
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/pve1/storage/local/content":
			mark(r.URL.Path)
			_ = json.NewEncoder(w).Encode(storageContentResponse{Data: []storageContentEntry{
				{VolID: "local:backup/a-old", Content: "backup", VMID: 101, CTime: 1000, Size: 2, Format: "tar"},
				{VolID: "local:snippet/x", Content: "snippet", VMID: 101, CTime: 999}, // wrong content type
			}})
		case "/api2/json/nodes/pve1/storage/nfs-bulk/content":
			mark(r.URL.Path)
			_ = json.NewEncoder(w).Encode(storageContentResponse{Data: []storageContentEntry{
				{VolID: "nfs-bulk:backup/b-new", Content: "backup", VMID: 101, CTime: 3000, Size: 4, Format: "zst"},
				{VolID: "nfs-bulk:backup/other-vm", Content: "backup", VMID: 999, CTime: 2500}, // wrong vmid
			}})
		case "/api2/json/nodes/pve1/storage/pbs/content":
			mark(r.URL.Path)
			_ = json.NewEncoder(w).Encode(storageContentResponse{Data: []storageContentEntry{
				{VolID: "pbs:backup/c-mid", Content: "backup", VMID: 101, CTime: 2000, Size: 8, Format: "pbs"},
			}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	backups, err := client.ListBackups(context.Background(), node, storages, 101)
	if err != nil {
		t.Fatalf("ListBackups() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, storage := range storages {
		path := "/api2/json/nodes/" + node + "/storage/" + storage + "/content"
		if !pathsSeen[path] {
			t.Errorf("storage %q was never fetched (fan-out incomplete)", storage)
		}
	}

	wantVolidOrder := []string{"nfs-bulk:backup/b-new", "pbs:backup/c-mid", "local:backup/a-old"}
	if len(backups) != len(wantVolidOrder) {
		t.Fatalf("ListBackups() = %d backups, want %d: %+v", len(backups), len(wantVolidOrder), backups)
	}
	for i, b := range backups {
		if b.VolID != wantVolidOrder[i] {
			t.Errorf("backups[%d].VolID = %q, want %q (newest-first sort)", i, b.VolID, wantVolidOrder[i])
		}
		if b.Storage == "" || b.Node != node {
			t.Errorf("backups[%d] missing storage/node: %+v", i, b)
		}
	}
}

func TestListBackupsEmptyStorages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request with no storages: %q", r.URL.Path)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	backups, err := client.ListBackups(context.Background(), "pve1", nil, 101)
	if err != nil {
		t.Fatalf("ListBackups() error = %v, want nil for empty storages", err)
	}
	if backups != nil {
		t.Errorf("ListBackups() = %v, want nil slice for empty storages", backups)
	}
}

// TestListBackupsReportsFirstErrorInStorageOrder confirms the concurrent
// merge surfaces the lowest-index storage's error first, matching the
// original sequential version's in-order error reporting — not whichever
// goroutine happened to finish first.
func TestListBackupsReportsFirstErrorInStorageOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/pve1/storage/local/content":
			// First storage errors; second is healthy.
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintln(w, `{"data":null}`)
		case "/api2/json/nodes/pve1/storage/rpool/content":
			_ = json.NewEncoder(w).Encode(storageContentResponse{})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	_, err := client.ListBackups(context.Background(), "pve1", []string{"local", "rpool"}, 101)
	if err == nil {
		t.Fatal("ListBackups() error = nil, want an error from the failing storage")
	}
	if got, want := err.Error(), "listing content on pve1/local"; got[:len(want)] != want {
		t.Errorf("ListBackups() error = %q, want it to name the first (local) storage", err.Error())
	}
}
