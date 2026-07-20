package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestQmCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"qm"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("qm") error = %v`, err)
	}
	if found.Use != "qm" {
		t.Errorf(`Find("qm").Use = %q, want "qm"`, found.Use)
	}
}

func TestQmListCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"qm", "list"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("qm", "list") error = %v`, err)
	}
	if found.Name() != "list" {
		t.Errorf(`Find("qm", "list").Name() = %q, want "list"`, found.Name())
	}
}

func TestRenderVMList(t *testing.T) {
	vms := []api.VM{
		{VMID: 201, Name: "opnsense", Node: "pve1", Status: "running"},
		{VMID: 202, Name: "media-server", Node: "pve2", Status: "stopped"},
	}

	got := renderVMList(vms)

	for _, want := range []string{"VMID", "NAME", "NODE", "STATUS", "201", "opnsense", "pve1", "running", "202", "media-server", "pve2", "stopped"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderVMList() = %q, want it to contain %q", got, want)
		}
	}
}

func TestRunQmList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"type": "qemu", "vmid": 201, "name": "opnsense", "node": "pve1", "status": "running"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if err := runQmList(client, ""); err != nil {
		t.Fatalf("runQmList() error = %v", err)
	}
}

func TestRunQmListFiltersByNode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"type": "qemu", "vmid": 201, "name": "opnsense", "node": "pve1", "status": "running"},
				{"type": "qemu", "vmid": 202, "name": "media-server", "node": "pve2", "status": "stopped"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = w
	runErr := runQmList(client, "pve2")
	os.Stdout = origStdout
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if runErr != nil {
		t.Fatalf("runQmList() error = %v", runErr)
	}
	got := buf.String()
	if strings.Contains(got, "opnsense") {
		t.Errorf("runQmList(node=pve2) = %q, should not contain filtered-out VM", got)
	}
	if !strings.Contains(got, "media-server") {
		t.Errorf("runQmList(node=pve2) = %q, want it to contain media-server", got)
	}
}
