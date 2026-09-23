package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestConfigUpdateUsesDigestAndDeleteParameter(t *testing.T) {
	for _, tc := range []struct{ guest, action, field, value, wantBody string }{
		{"lxc", "set", "description", "hello world", "description=hello+world"},
		{"qemu", "set", "description", "hello world", "description=hello+world"},
		{"lxc", "unset", "description", "", "delete=description"},
		{"qemu", "unset", "description", "", "delete=description"},
	} {
		t.Run(tc.guest+"/"+tc.action, func(t *testing.T) {
			var body string
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"digest": "abc", "description": "old"}})
					return
				}
				if r.Method != http.MethodPut || !strings.Contains(r.URL.Path, "/"+tc.guest+"/101/config") {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				_ = r.ParseForm()
				body = r.Form.Encode()
				_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
			}))
			defer s.Close()
			client := api.NewClient(s.URL, "user@pve!test", "secret", true)
			if err := updateConfigField(client, tc.guest, "pve1", 101, tc.action, tc.field, tc.value); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(body, "digest=abc") || !strings.Contains(body, tc.wantBody) {
				t.Errorf("PUT form = %q", body)
			}
		})
	}
}

func TestConfigUpdateRejectsUnsafeFieldsBeforeRequest(t *testing.T) {
	for _, tc := range []struct{ guest, field string }{{"lxc", "lxc.mount.entry"}, {"lxc", "rootfs"}, {"lxc", "revert"}, {"qemu", "scsi0"}, {"qemu", "vmstate"}, {"qemu", "digest"}, {"qemu", "delete"}, {"qemu", "revert"}, {"qemu", "skiplock"}, {"qemu", "force"}, {"qemu", "sshkeys"}} {
		t.Run(tc.guest+"/"+tc.field, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
			defer s.Close()
			client := api.NewClient(s.URL, "user@pve!test", "secret", true)
			if err := updateConfigField(client, tc.guest, "pve1", 101, "unset", tc.field, ""); err == nil {
				t.Fatal("expected unsafe field rejection")
			}
			if calls != 0 {
				t.Errorf("made %d requests", calls)
			}
		})
	}
}
