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

func TestRenameContainerSendsHostnameAndDigest(t *testing.T) {
	var gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"digest": "abc123", "hostname": "web01"},
			})
		case http.MethodPut:
			gotPath = r.URL.Path
			body, _ := io.ReadAll(r.Body)
			gotBody = string(body)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	c := api.Container{VMID: 101, Name: "web01", Node: "pve1"}

	if err := renameContainer(client, c, "web02"); err != nil {
		t.Fatalf("renameContainer() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/lxc/101/config" {
		t.Errorf("PUT path = %q", gotPath)
	}
	if !strings.Contains(gotBody, "hostname=web02") {
		t.Errorf("PUT body = %q, want it to contain hostname=web02", gotBody)
	}
	if !strings.Contains(gotBody, "digest=abc123") {
		t.Errorf("PUT body = %q, want it to contain digest=abc123", gotBody)
	}
}

func TestRenameContainerDigestMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"digest": "stale", "hostname": "web01"},
			})
		case http.MethodPut:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "digest mismatch"})
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	c := api.Container{VMID: 101, Name: "web01", Node: "pve1"}

	err := renameContainer(client, c, "web02")
	if err == nil {
		t.Fatal("renameContainer() error = nil, want digest mismatch error")
	}
	if !strings.Contains(err.Error(), "config changed elsewhere") {
		t.Errorf("renameContainer() error = %q, want it to mention 'config changed elsewhere'", err.Error())
	}
}

func TestRenameVMSendsNameAndDigest(t *testing.T) {
	var gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"digest": "abc123", "name": "web01"},
			})
		case http.MethodPut:
			gotPath = r.URL.Path
			body, _ := io.ReadAll(r.Body)
			gotBody = string(body)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	v := api.VM{VMID: 201, Name: "web01", Node: "pve1"}

	if err := renameVM(client, v, "web02"); err != nil {
		t.Fatalf("renameVM() error = %v", err)
	}
	if gotPath != "/api2/json/nodes/pve1/qemu/201/config" {
		t.Errorf("PUT path = %q", gotPath)
	}
	if !strings.Contains(gotBody, "name=web02") {
		t.Errorf("PUT body = %q, want it to contain name=web02", gotBody)
	}
	if !strings.Contains(gotBody, "digest=abc123") {
		t.Errorf("PUT body = %q, want it to contain digest=abc123", gotBody)
	}
}

func TestRenameVMDigestMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"digest": "stale", "name": "web01"},
			})
		case http.MethodPut:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "digest mismatch"})
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	v := api.VM{VMID: 201, Name: "web01", Node: "pve1"}

	err := renameVM(client, v, "web02")
	if err == nil {
		t.Fatal("renameVM() error = nil, want digest mismatch error")
	}
	if !strings.Contains(err.Error(), "config changed elsewhere") {
		t.Errorf("renameVM() error = %q, want it to mention 'config changed elsewhere'", err.Error())
	}
}

func TestRenameCommandsRegistered(t *testing.T) {
	for _, args := range [][]string{{"ct", "rename"}, {"qm", "rename"}} {
		found, _, err := rootCmd.Find(args)
		if err != nil {
			t.Errorf("rootCmd.Find(%v) error = %v", args, err)
			continue
		}
		if found.Name() != "rename" {
			t.Errorf("Find(%v).Name() = %q, want %q", args, found.Name(), "rename")
		}
	}
}

func TestDispatchActionRename(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"digest": "abc123", "hostname": "web01"},
			})
		case http.MethodPut:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	c := api.Container{VMID: 101, Name: "web01", Node: "pve1"}

	ctRenameName = "web02"
	defer func() { ctRenameName = "" }()

	if err := dispatchAction(client, "rename", c); err != nil {
		t.Errorf("dispatchAction(rename) error = %v", err)
	}
}

func TestDispatchVMActionRename(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"digest": "abc123", "name": "web01"},
			})
		case http.MethodPut:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": nil})
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	v := api.VM{VMID: 201, Name: "web01", Node: "pve1"}

	qmRenameName = "web02"
	defer func() { qmRenameName = "" }()

	if err := dispatchVMAction(client, "rename", v); err != nil {
		t.Errorf("dispatchVMAction(rename) error = %v", err)
	}
}
