package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

// TestResolveContainerWithArgSkipsPicker confirms the core of the
// "pvectl ct <action> <name-or-vmid> shouldn't open a selector" request:
// given an identifier, resolveContainer must resolve it directly via
// findContainer without ever invoking the interactive picker (which
// would hang waiting on a TTY in this test).
func TestResolveContainerWithArgSkipsPicker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"type": "lxc", "vmid": 101, "name": "janus", "node": "pve1", "status": "running"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	c, err := resolveContainer(client, []string{"janus"})
	if err != nil {
		t.Fatalf("resolveContainer() error = %v", err)
	}
	if c.VMID != 101 {
		t.Errorf("resolveContainer().VMID = %d, want 101", c.VMID)
	}
}

func TestResolveContainerWithUnknownArgErrorsWithoutPicker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if _, err := resolveContainer(client, []string{"nonexistent"}); err == nil {
		t.Error("resolveContainer() error = nil, want an error for an unknown name")
	}
}

func TestResolveVMWithArgSkipsPicker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"type": "qemu", "vmid": 201, "name": "opnsense", "node": "pve1", "status": "running"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	v, err := resolveVM(client, []string{"opnsense"})
	if err != nil {
		t.Fatalf("resolveVM() error = %v", err)
	}
	if v.VMID != 201 {
		t.Errorf("resolveVM().VMID = %d, want 201", v.VMID)
	}
}

func TestResolveVMWithUnknownArgErrorsWithoutPicker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if _, err := resolveVM(client, []string{"nonexistent"}); err == nil {
		t.Error("resolveVM() error = nil, want an error for an unknown name")
	}
}
