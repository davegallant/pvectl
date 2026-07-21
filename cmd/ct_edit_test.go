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

func TestApplyEditNoChanges(t *testing.T) {
	client := api.NewClient("https://unused.invalid:8006", "user@pve!test", "secret", true)
	original := api.Config{
		Digest: "abc123",
		Fields: map[string]string{"hostname": "web01", "cores": "2"},
	}

	if err := applyEdit(client, "pve1", 101, original, "hostname: web01\ncores: 2\n"); err != nil {
		t.Fatalf("applyEdit() error = %v, want nil (no PUT should be attempted)", err)
	}
}

// TestApplyEditWarnsOnRemovedFields documents that field deletions made in
// $EDITOR are intentionally unsupported: applyEdit only ever PUTs
// diff.Changed, never issues a delete for diff.Removed. When a deletion is
// the only edit made, no PUT should be attempted (matching
// TestApplyEditNoChanges), and a warning is printed instead of silently
// dropping the deletion with no explanation.
func TestApplyEditWarnsOnRemovedFields(t *testing.T) {
	putCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		putCalled = true
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	original := api.Config{
		Digest: "abc123",
		Fields: map[string]string{"hostname": "web01", "cores": "2"},
	}

	// "cores" is deleted in the edited text; "hostname" is unchanged.
	if err := applyEdit(client, "pve1", 101, original, "hostname: web01\n"); err != nil {
		t.Fatalf("applyEdit() error = %v, want nil", err)
	}
	if putCalled {
		t.Error("applyEdit() called PutConfig, want no PUT when only a field deletion occurred")
	}
}

func TestApplyEditSendsChangedFields(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	original := api.Config{
		Digest: "abc123",
		Fields: map[string]string{"hostname": "web01", "cores": "2"},
	}

	if err := applyEdit(client, "pve1", 101, original, "hostname: web01\ncores: 4\n"); err != nil {
		t.Fatalf("applyEdit() error = %v", err)
	}
	if !strings.Contains(gotBody, "cores=4") {
		t.Errorf("PUT body = %q, want it to contain cores=4", gotBody)
	}
	if !strings.Contains(gotBody, "digest=abc123") {
		t.Errorf("PUT body = %q, want it to contain digest=abc123", gotBody)
	}
	if strings.Contains(gotBody, "hostname=") {
		t.Errorf("PUT body = %q, should not include the unchanged hostname field", gotBody)
	}
}

// TestApplyEditIgnoresRawLXCLines documents that "lxc.*" passthrough lines
// (shown in the editor for context, rendered from api.Config.RawLXC, which
// has no dedicated Proxmox API parameter and can't round-trip through a
// map) are never sent to PutConfig, even if present in the edited text —
// only a real change to a genuine Fields key should trigger a PUT.
func TestApplyEditIgnoresRawLXCLines(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	original := api.Config{
		Digest: "abc123",
		Fields: map[string]string{"hostname": "web01", "cores": "2"},
	}

	editedText := "cores: 4\nhostname: web01\nlxc.cgroup2.devices.allow: c 10:200 rwm\nlxc.mount.entry: /dev/net dev/net none bind,create=dir\n"
	if err := applyEdit(client, "pve1", 101, original, editedText); err != nil {
		t.Fatalf("applyEdit() error = %v", err)
	}
	if strings.Contains(gotBody, "lxc") {
		t.Errorf("PUT body = %q, must not include any lxc.* keys", gotBody)
	}
	if !strings.Contains(gotBody, "cores=4") {
		t.Errorf("PUT body = %q, want it to still contain the real cores=4 change", gotBody)
	}
}

// TestApplyEditNoChangesWithOnlyRawLXCLines documents that leaving the
// "lxc.*" context lines untouched (with no real Fields change) must not
// trigger a PUT at all.
func TestApplyEditNoChangesWithOnlyRawLXCLines(t *testing.T) {
	putCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		putCalled = true
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	original := api.Config{
		Digest: "abc123",
		Fields: map[string]string{"hostname": "web01"},
	}

	editedText := "hostname: web01\nlxc.cgroup2.devices.allow: c 10:200 rwm\n"
	if err := applyEdit(client, "pve1", 101, original, editedText); err != nil {
		t.Fatalf("applyEdit() error = %v", err)
	}
	if putCalled {
		t.Error("applyEdit() called PutConfig, want no PUT when only lxc.* context lines are present")
	}
}

func TestApplyEditDigestMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "digest mismatch"})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	original := api.Config{
		Digest: "stale",
		Fields: map[string]string{"hostname": "web01"},
	}

	err := applyEdit(client, "pve1", 101, original, "hostname: web02\n")
	if err == nil {
		t.Fatal("applyEdit() error = nil, want digest mismatch error")
	}
	if !strings.Contains(err.Error(), "config changed elsewhere") {
		t.Errorf("applyEdit() error = %q, want it to mention 'config changed elsewhere'", err.Error())
	}
}

func TestEditCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"ct", "config", "edit"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("ct", "config", "edit") error = %v`, err)
	}
	if found.Name() != "edit" {
		t.Errorf(`Find("ct", "config", "edit").Name() = %q, want "edit"`, found.Name())
	}
}
