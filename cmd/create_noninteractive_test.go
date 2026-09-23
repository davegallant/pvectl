package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestNonInteractiveCreateReportsAllMissingFields(t *testing.T) {
	for _, tc := range []struct {
		guest  string
		inputs map[string]string
		want   []string
	}{
		{"ct", map[string]string{"node": "", "template": "", "storage": "", "hostname": ""}, []string{"--node", "--template", "--storage", "--hostname"}},
		{"qm", map[string]string{"node": "", "storage": "", "name": ""}, []string{"--node", "--storage", "--name"}},
	} {
		t.Run(tc.guest, func(t *testing.T) {
			err := validateNonInteractiveCreate(tc.inputs)
			if err == nil {
				t.Fatal("expected missing fields error")
			}
			for _, flag := range tc.want {
				if !strings.Contains(err.Error(), flag) {
					t.Errorf("error %q missing %s", err, flag)
				}
			}
		})
	}
	if err := validateNonInteractiveCreate(map[string]string{"node": "pve1", "name": "vm", "storage": "local-lvm"}); err != nil {
		t.Fatal(err)
	}
}

func TestNonInteractiveCreateRejectsPasswordPrompt(t *testing.T) {
	if err := validateNonInteractivePassword("-"); err == nil || !strings.Contains(err.Error(), "--cipassword") {
		t.Fatalf("password prompt error = %v", err)
	}
	if err := validateNonInteractivePassword("secret"); err != nil {
		t.Fatal(err)
	}
}

func TestNonInteractiveCreateWithClosedStdin(t *testing.T) {
	closed, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = closed.Close() }()
	previousStdin := os.Stdin
	os.Stdin = closed
	defer func() { os.Stdin = previousStdin }()

	for _, guest := range []string{"ct", "qm"} {
		t.Run(guest, func(t *testing.T) {
			posts := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					if err := r.ParseForm(); err != nil {
						t.Error(err)
					}
					if guest == "qm" && r.Form.Has("ide2") {
						t.Error("non-interactive VM create unexpectedly attached an ISO")
					}
					posts++
					_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:pve1:create"})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
			}))
			defer s.Close()
			client := api.NewClient(s.URL, "user@pve!test", "secret", true)
			if guest == "ct" {
				ctCreateNonInteractive = true
				ctCreateNode, ctCreateTemplate, ctCreateStorage, ctCreateHostname, ctCreateVMID = "pve1", "tpl", "local-lvm", "web", 101
				defer func() {
					ctCreateNonInteractive = false
					ctCreateNode, ctCreateTemplate, ctCreateStorage, ctCreateHostname, ctCreateVMID = "", "", "", "", 0
				}()
				ctCreateStorage = ""
				if err := runCtCreate(client, false); err == nil || !strings.Contains(err.Error(), "--storage") {
					t.Fatalf("missing storage error = %v", err)
				}
				if posts != 0 {
					t.Fatalf("incomplete create sent %d POSTs", posts)
				}
				ctCreateStorage = "local-lvm"
				if err := runCtCreate(client, false); err != nil {
					t.Fatal(err)
				}
			} else {
				qmCreateNonInteractive = true
				qmCreateNode, qmCreateStorage, qmCreateName, qmCreateVMID = "pve1", "local-lvm", "web", 101
				defer func() {
					qmCreateNonInteractive = false
					qmCreateNode, qmCreateStorage, qmCreateName, qmCreateVMID = "", "", "", 0
				}()
				qmCreateStorage = ""
				if err := runQmCreate(client, false); err == nil || !strings.Contains(err.Error(), "--storage") {
					t.Fatalf("missing storage error = %v", err)
				}
				if posts != 0 {
					t.Fatalf("incomplete create sent %d POSTs", posts)
				}
				qmCreateStorage = "local-lvm"
				if err := runQmCreate(client, false); err != nil {
					t.Fatal(err)
				}
			}
			if posts != 1 {
				t.Errorf("complete create sent %d POSTs, want 1", posts)
			}
		})
	}
}
