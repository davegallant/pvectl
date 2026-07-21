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

// TestRunCtCloneExplicitNewID confirms the direct path (--newid set)
// never touches NextID: findContainer resolves the source from the
// cluster resource list, then CloneContainer is called with node/vmid
// straight from the resolved container, matching runCtMigrate's
// "resolve, then dispatch" shape.
func TestRunCtCloneExplicitNewID(t *testing.T) {
	var gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/cluster/resources"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"type": "lxc", "vmid": 101, "name": "web01", "node": "pve1", "status": "running"},
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

	ctCloneNewID = 105
	ctCloneHostname = "web01-clone"
	defer func() {
		ctCloneNewID = 0
		ctCloneHostname = ""
	}()

	if err := runCtClone(client, []string{"web01"}); err != nil {
		t.Fatalf("runCtClone() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/lxc/101/clone" {
		t.Errorf("clone path = %q, want /api2/json/nodes/pve1/lxc/101/clone", gotPath)
	}
	values, err := url.ParseQuery(gotBody)
	if err != nil {
		t.Fatalf("ParseQuery(%q) error = %v", gotBody, err)
	}
	if values.Get("newid") != "105" || values.Get("hostname") != "web01-clone" {
		t.Errorf("body = %q, want newid=105&hostname=web01-clone", gotBody)
	}
}

// TestRunCtCloneAutoNewID confirms newid=0 (the flag default) falls back
// to NextID, the same auto-assign convention as ct create's --vmid.
func TestRunCtCloneAutoNewID(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/cluster/resources"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"type": "lxc", "vmid": 101, "name": "web01", "node": "pve1", "status": "running"},
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

	ctCloneNewID = 0
	if err := runCtClone(client, []string{"web01"}); err != nil {
		t.Fatalf("runCtClone() error = %v", err)
	}
	values, err := url.ParseQuery(gotBody)
	if err != nil {
		t.Fatalf("ParseQuery(%q) error = %v", gotBody, err)
	}
	if values.Get("newid") != "999" {
		t.Errorf(`body["newid"] = %q, want "999" from NextID`, values.Get("newid"))
	}
}

func TestCloneCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"ct", "clone"}); err != nil {
		t.Errorf("rootCmd.Find([ct clone]) error = %v", err)
	}
}
