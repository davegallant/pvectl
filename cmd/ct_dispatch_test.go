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

func TestCtCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"ct"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("ct") error = %v`, err)
	}
	if found.Use != "ct" {
		t.Errorf(`Find("ct").Use = %q, want "ct"`, found.Use)
	}
}

func TestCtListCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"ct", "list"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("ct", "list") error = %v`, err)
	}
	if found.Name() != "list" {
		t.Errorf(`Find("ct", "list").Name() = %q, want "list"`, found.Name())
	}
}

func TestRenderContainerList(t *testing.T) {
	containers := []api.Container{
		{VMID: 101, Name: "web01", Node: "pve1", Status: "running"},
		{VMID: 102, Name: "db01", Node: "pve2", Status: "stopped"},
	}

	got := renderContainerList(containers)

	for _, want := range []string{"VMID", "NAME", "NODE", "STATUS", "101", "web01", "pve1", "running", "102", "db01", "pve2", "stopped"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderContainerList() = %q, want it to contain %q", got, want)
		}
	}
}

func TestRunCtList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"type": "lxc", "vmid": 101, "name": "web01", "node": "pve1", "status": "running"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if err := runCtList(client, ""); err != nil {
		t.Fatalf("runCtList() error = %v", err)
	}
}

func TestRunCtListFiltersByNode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"type": "lxc", "vmid": 101, "name": "web01", "node": "pve1", "status": "running"},
				{"type": "lxc", "vmid": 102, "name": "db01", "node": "pve2", "status": "stopped"},
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
	runErr := runCtList(client, "pve2")
	os.Stdout = origStdout
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if runErr != nil {
		t.Fatalf("runCtList() error = %v", runErr)
	}
	got := buf.String()
	if strings.Contains(got, "web01") {
		t.Errorf("runCtList(node=pve2) = %q, should not contain filtered-out container", got)
	}
	if !strings.Contains(got, "db01") {
		t.Errorf("runCtList(node=pve2) = %q, want it to contain db01", got)
	}
}
