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

func TestApplyVMEditNoChanges(t *testing.T) {
	client := api.NewClient("https://unused.invalid:8006", "user@pve!test", "secret", true)
	original := api.VMConfig{
		Digest: "abc123",
		Fields: map[string]string{"name": "web01", "cores": "2"},
	}

	if err := applyVMEdit(client, "pve1", 201, original, "name: web01\ncores: 2\n"); err != nil {
		t.Fatalf("applyVMEdit() error = %v, want nil (no PUT should be attempted)", err)
	}
}

func TestApplyVMEditWarnsOnRemovedFields(t *testing.T) {
	putCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		putCalled = true
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	original := api.VMConfig{
		Digest: "abc123",
		Fields: map[string]string{"name": "web01", "cores": "2"},
	}

	// "cores" is deleted in the edited text; "name" is unchanged.
	if err := applyVMEdit(client, "pve1", 201, original, "name: web01\n"); err != nil {
		t.Fatalf("applyVMEdit() error = %v, want nil", err)
	}
	if putCalled {
		t.Error("applyVMEdit() called PutVMConfig, want no PUT when only a field deletion occurred")
	}
}

func TestApplyVMEditSendsChangedFields(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	original := api.VMConfig{
		Digest: "abc123",
		Fields: map[string]string{"name": "web01", "cores": "2"},
	}

	if err := applyVMEdit(client, "pve1", 201, original, "name: web01\ncores: 4\n"); err != nil {
		t.Fatalf("applyVMEdit() error = %v", err)
	}
	if !strings.Contains(gotBody, "cores=4") {
		t.Errorf("PUT body = %q, want it to contain cores=4", gotBody)
	}
	if !strings.Contains(gotBody, "digest=abc123") {
		t.Errorf("PUT body = %q, want it to contain digest=abc123", gotBody)
	}
	if strings.Contains(gotBody, "name=") {
		t.Errorf("PUT body = %q, should not include the unchanged name field", gotBody)
	}
}

func TestApplyVMEditDigestMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "digest mismatch"})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	original := api.VMConfig{
		Digest: "stale",
		Fields: map[string]string{"name": "web01"},
	}

	err := applyVMEdit(client, "pve1", 201, original, "name: web02\n")
	if err == nil {
		t.Fatal("applyVMEdit() error = nil, want digest mismatch error")
	}
	if !strings.Contains(err.Error(), "config changed elsewhere") {
		t.Errorf("applyVMEdit() error = %q, want it to mention 'config changed elsewhere'", err.Error())
	}
}

func TestQmEditCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"qm", "config", "edit"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("qm", "config", "edit") error = %v`, err)
	}
	if found.Name() != "edit" {
		t.Errorf(`Find("qm", "config", "edit").Name() = %q, want "edit"`, found.Name())
	}
}
