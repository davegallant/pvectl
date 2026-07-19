package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/davegallant/pvectl/internal/config"
	"github.com/davegallant/pvectl/internal/secrets"
	"github.com/spf13/cobra"
)

func TestContainerNames(t *testing.T) {
	containers := []api.Container{
		{VMID: 101, Name: "janus"},
		{VMID: 102, Name: "web01"},
	}
	got := containerNames(containers)
	want := []string{"janus", "web01"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("containerNames() = %v, want %v", got, want)
	}
}

func TestVMNames(t *testing.T) {
	vms := []api.VM{
		{VMID: 201, Name: "opnsense"},
	}
	got := vmNames(vms)
	want := []string{"opnsense"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("vmNames() = %v, want %v", got, want)
	}
}

// TestCompleteContainerNamesSkipsWhenArgAlreadyGiven confirms the
// name-or-vmid slot only completes when empty — with one already given
// (or any later flag value), there's nothing left to suggest, and this
// must not touch the network to decide that.
func TestCompleteContainerNamesSkipsWhenArgAlreadyGiven(t *testing.T) {
	got, directive := completeContainerNames(&cobra.Command{}, []string{"janus"}, "")
	if got != nil {
		t.Errorf("completeContainerNames() = %v, want nil", got)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("completeContainerNames() directive = %v, want NoFileComp", directive)
	}
}

func TestCompleteVMNamesSkipsWhenArgAlreadyGiven(t *testing.T) {
	got, directive := completeVMNames(&cobra.Command{}, []string{"opnsense"}, "")
	if got != nil {
		t.Errorf("completeVMNames() = %v, want nil", got)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("completeVMNames() directive = %v, want NoFileComp", directive)
	}
}

// withFakeClient points loadClient() at a fake config/secret store backed
// by srv, so completeContainerNames/completeVMNames (which call loadClient
// internally, same as every command's RunE) can be exercised end-to-end.
func withFakeClient(t *testing.T, srv *httptest.Server) {
	t.Helper()
	origDir := config.ConfigDir
	tmpDir := t.TempDir()
	config.ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { config.ConfigDir = origDir })

	fake := secrets.NewFakeStore()
	keyringStore = fake
	t.Cleanup(func() { keyringStore = secrets.KeyringStore{} })

	if err := config.Save(&config.Config{Host: srv.URL, TokenID: "user@pve!pvectl"}); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}
	if err := fake.Set(srv.URL, "s3cr3t"); err != nil {
		t.Fatalf("fake.Set() error = %v", err)
	}
}

func TestCompleteContainerNamesFetchesLiveList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"type": "lxc", "vmid": 101, "name": "janus", "node": "pve1", "status": "running"},
			},
		})
	}))
	defer server.Close()
	withFakeClient(t, server)

	got, directive := completeContainerNames(&cobra.Command{}, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("completeContainerNames() directive = %v, want NoFileComp", directive)
	}
	if len(got) != 1 || got[0] != "janus" {
		t.Errorf("completeContainerNames() = %v, want [janus]", got)
	}
}

func TestCompleteVMNamesFetchesLiveList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"type": "qemu", "vmid": 201, "name": "opnsense", "node": "pve1", "status": "running"},
			},
		})
	}))
	defer server.Close()
	withFakeClient(t, server)

	got, directive := completeVMNames(&cobra.Command{}, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("completeVMNames() directive = %v, want NoFileComp", directive)
	}
	if len(got) != 1 || got[0] != "opnsense" {
		t.Errorf("completeVMNames() = %v, want [opnsense]", got)
	}
}

func TestCompleteContainerNamesErrorsWithoutSetup(t *testing.T) {
	origDir := config.ConfigDir
	tmpDir := t.TempDir()
	config.ConfigDir = func() (string, error) { return tmpDir, nil }
	t.Cleanup(func() { config.ConfigDir = origDir })

	got, directive := completeContainerNames(&cobra.Command{}, nil, "")
	if got != nil {
		t.Errorf("completeContainerNames() = %v, want nil", got)
	}
	if directive != cobra.ShellCompDirectiveError {
		t.Errorf("completeContainerNames() directive = %v, want Error", directive)
	}
}
