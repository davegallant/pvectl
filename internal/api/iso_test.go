package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListISOsFansOutAcrossStorages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/pve1/storage/local/content":
			_ = json.NewEncoder(w).Encode(storageContentResponse{Data: []storageContentEntry{
				{VolID: "local:iso/ubuntu-24.04.4-live-server-amd64.iso", Content: "iso", Size: 100},
				{VolID: "local:backup/a", Content: "backup"}, // wrong content type
			}})
		case "/api2/json/nodes/pve1/storage/nfs-bulk/content":
			_ = json.NewEncoder(w).Encode(storageContentResponse{Data: []storageContentEntry{
				{VolID: "nfs-bulk:iso/debian-12.7.0-amd64-netinst.iso", Content: "iso", Size: 200},
			}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	isos, err := client.ListISOs(context.Background(), "pve1", []string{"local", "nfs-bulk"})
	if err != nil {
		t.Fatalf("ListISOs() error = %v", err)
	}
	if len(isos) != 2 {
		t.Fatalf("ListISOs() = %+v, want 2 isos", isos)
	}
	// Sorted by volid: "local:..." < "nfs-bulk:...".
	if isos[0].VolID != "local:iso/ubuntu-24.04.4-live-server-amd64.iso" || isos[0].Storage != "local" {
		t.Errorf("isos[0] = %+v", isos[0])
	}
	if isos[1].VolID != "nfs-bulk:iso/debian-12.7.0-amd64-netinst.iso" || isos[1].Storage != "nfs-bulk" {
		t.Errorf("isos[1] = %+v", isos[1])
	}
}

func TestListISOsEmptyStorages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request with no storages: %q", r.URL.Path)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	isos, err := client.ListISOs(context.Background(), "pve1", nil)
	if err != nil {
		t.Fatalf("ListISOs() error = %v, want nil for empty storages", err)
	}
	if isos != nil {
		t.Errorf("ListISOs() = %v, want nil slice for empty storages", isos)
	}
}
