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
)

// TestRunQmCloneExplicitNewID mirrors TestRunCtCloneExplicitNewID for
// QEMU VMs.
func TestRunQmCloneExplicitNewID(t *testing.T) {
	var gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/cluster/resources"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"type": "qemu", "vmid": 201, "name": "web01", "node": "pve1", "status": "running"},
				},
			})
		case strings.Contains(r.URL.Path, "/clone"):
			gotPath = r.URL.Path
			body, _ := io.ReadAll(r.Body)
			gotBody = string(body)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:clone"})
		case strings.Contains(r.URL.Path, "/tasks/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)

	qmCloneNewID = 205
	qmCloneName = "web01-clone"
	defer func() {
		qmCloneNewID = 0
		qmCloneName = ""
	}()

	if err := runQmClone(client, []string{"web01"}); err != nil {
		t.Fatalf("runQmClone() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/qemu/201/clone" {
		t.Errorf("clone path = %q, want /api2/json/nodes/pve1/qemu/201/clone", gotPath)
	}
	values, err := url.ParseQuery(gotBody)
	if err != nil {
		t.Fatalf("ParseQuery(%q) error = %v", gotBody, err)
	}
	if values.Get("newid") != "205" || values.Get("name") != "web01-clone" {
		t.Errorf("body = %q, want newid=205&name=web01-clone", gotBody)
	}
}

// TestRunQmCloneAutoNewID mirrors TestRunCtCloneAutoNewID for QEMU VMs.
func TestRunQmCloneAutoNewID(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/cluster/resources"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"type": "qemu", "vmid": 201, "name": "web01", "node": "pve1", "status": "running"},
				},
			})
		case strings.Contains(r.URL.Path, "/cluster/nextid"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "999"})
		case strings.Contains(r.URL.Path, "/clone"):
			body, _ := io.ReadAll(r.Body)
			gotBody = string(body)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:clone"})
		case strings.Contains(r.URL.Path, "/tasks/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)

	qmCloneNewID = 0
	if err := runQmClone(client, []string{"web01"}); err != nil {
		t.Fatalf("runQmClone() error = %v", err)
	}
	values, err := url.ParseQuery(gotBody)
	if err != nil {
		t.Fatalf("ParseQuery(%q) error = %v", gotBody, err)
	}
	if values.Get("newid") != "999" {
		t.Errorf(`body["newid"] = %q, want "999" from NextID`, values.Get("newid"))
	}
}

func TestQmCloneCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"qm", "clone"}); err != nil {
		t.Errorf("rootCmd.Find([qm clone]) error = %v", err)
	}
}
