package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListTemplatesFansOutAcrossStorages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/pve1/storage/local/content":
			_ = json.NewEncoder(w).Encode(storageContentResponse{Data: []storageContentEntry{
				{VolID: "local:vztmpl/ubuntu-24.04-standard_24.04-2_amd64.tar.zst", Content: "vztmpl", Size: 100},
				{VolID: "local:backup/a", Content: "backup"}, // wrong content type
			}})
		case "/api2/json/nodes/pve1/storage/nfs-bulk/content":
			_ = json.NewEncoder(w).Encode(storageContentResponse{Data: []storageContentEntry{
				{VolID: "nfs-bulk:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst", Content: "vztmpl", Size: 200},
			}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	templates, err := client.ListTemplates(context.Background(), "pve1", []string{"local", "nfs-bulk"})
	if err != nil {
		t.Fatalf("ListTemplates() error = %v", err)
	}
	if len(templates) != 2 {
		t.Fatalf("ListTemplates() = %+v, want 2 templates", templates)
	}
	// Sorted by volid: "local:..." < "nfs-bulk:...".
	if templates[0].VolID != "local:vztmpl/ubuntu-24.04-standard_24.04-2_amd64.tar.zst" || templates[0].Storage != "local" {
		t.Errorf("templates[0] = %+v", templates[0])
	}
	if templates[1].VolID != "nfs-bulk:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst" || templates[1].Storage != "nfs-bulk" {
		t.Errorf("templates[1] = %+v", templates[1])
	}
}

func TestListTemplatesEmptyStorages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request with no storages: %q", r.URL.Path)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	templates, err := client.ListTemplates(context.Background(), "pve1", nil)
	if err != nil {
		t.Fatalf("ListTemplates() error = %v, want nil for empty storages", err)
	}
	if templates != nil {
		t.Errorf("ListTemplates() = %v, want nil slice for empty storages", templates)
	}
}
