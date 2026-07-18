package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListNodeStorages(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"storage": "local", "content": "vztmpl,backup,iso"},
				{"storage": "local-lvm", "content": "rootdir,images"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	storages, err := client.ListNodeStorages(context.Background(), "pve1")
	if err != nil {
		t.Fatalf("ListNodeStorages() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/storage" {
		t.Errorf("path = %q, want /api2/json/nodes/pve1/storage", gotPath)
	}
	if len(storages) != 2 {
		t.Fatalf("ListNodeStorages() = %+v, want 2 entries", storages)
	}
	if storages[0].Storage != "local" || storages[0].SupportsContent("rootdir") {
		t.Errorf("storages[0] = %+v, want local without rootdir support", storages[0])
	}
	if storages[1].Storage != "local-lvm" || !storages[1].SupportsContent("rootdir") {
		t.Errorf("storages[1] = %+v, want local-lvm with rootdir support", storages[1])
	}
}
