package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

// TestRunDeleteContainerSkipConfirm confirms skipConfirm=true never
// touches stdin and reaches the delete API directly — the analogous
// "flags make it fully non-interactive" path to
// TestRunCtMigrateDirectPath, but for delete's -y flag specifically.
func TestRunDeleteContainerSkipConfirm(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete:
			gotPath = r.URL.Path + "?" + r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:destroy"})
		case strings.Contains(r.URL.Path, "/tasks/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if err := runDeleteContainer(client, "pve1", 101, "web01", true, false, true); err != nil {
		t.Fatalf("runDeleteContainer() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/lxc/101?purge=1" {
		t.Errorf("delete path = %q, want /api2/json/nodes/pve1/lxc/101?purge=1", gotPath)
	}
}

func TestDeleteCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"ct", "delete"}); err != nil {
		t.Errorf("rootCmd.Find([ct delete]) error = %v", err)
	}
}
