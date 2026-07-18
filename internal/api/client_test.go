package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "PVEAPIToken=user@pve!test=secret123" {
			t.Errorf("Authorization header = %q, want token auth header", got)
		}
		if r.URL.Path != "/api2/json/version" {
			t.Errorf("request path = %q, want /api2/json/version", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]string{"version": "8.2.1"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret123", true)

	got, err := client.Version(context.Background())
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if got != "8.2.1" {
		t.Errorf("Version() = %q, want %q", got, "8.2.1")
	}
}

func TestClientListContainers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Real Proxmox servers have been observed returning an empty
		// "data" array when queried with "?type=vm" even though LXC
		// containers exist — that server-side filter is unreliable and
		// unnecessary, since we already filter client-side on each
		// resource's own "type" field below. Requests must hit the bare
		// endpoint with no query string.
		if r.URL.RawQuery != "" {
			t.Errorf("request query = %q, want no query string", r.URL.RawQuery)
		}
		if r.URL.Path != "/api2/json/cluster/resources" {
			t.Errorf("request path = %q, want /api2/json/cluster/resources", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"vmid": 101, "name": "web", "node": "pve1", "status": "running", "type": "lxc"},
				{"vmid": 102, "name": "db", "node": "pve2", "status": "stopped", "type": "lxc"},
				{"vmid": 200, "name": "vm1", "node": "pve1", "status": "running", "type": "qemu"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret123", true)

	got, err := client.ListContainers(context.Background())
	if err != nil {
		t.Fatalf("ListContainers() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListContainers() returned %d containers, want 2 (qemu type filtered out)", len(got))
	}
	if got[0].Name != "web" || got[1].Name != "db" {
		t.Errorf("ListContainers() = %+v, want web then db", got)
	}
}

func TestClientListContainersSortedByVMID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /cluster/resources is not guaranteed to return entries in VMID
		// order, so return them scrambled to prove ListContainers sorts
		// its own output rather than trusting the API's ordering.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"vmid": 103, "name": "web-02", "node": "pve2", "status": "running", "type": "lxc"},
				{"vmid": 101, "name": "web-01", "node": "pve1", "status": "running", "type": "lxc"},
				{"vmid": 102, "name": "db-01", "node": "pve1", "status": "stopped", "type": "lxc"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret123", true)

	got, err := client.ListContainers(context.Background())
	if err != nil {
		t.Fatalf("ListContainers() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ListContainers() returned %d containers, want 3", len(got))
	}
	wantOrder := []int{101, 102, 103}
	for i, vmid := range wantOrder {
		if got[i].VMID != vmid {
			t.Errorf("ListContainers()[%d].VMID = %d, want %d (sorted ascending)", i, got[i].VMID, vmid)
		}
	}
}

func TestClientAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "permission denied"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret123", true)

	_, err := client.Version(context.Background())
	if err == nil {
		t.Fatal("Version() error = nil, want error")
	}
	if err.Error() != "permission denied" {
		t.Errorf("Version() error = %q, want %q", err.Error(), "permission denied")
	}
}

// TestClientAPIErrorIncludesFieldErrors confirms a structured "errors" map
// isn't silently dropped when Message is also set — Proxmox's real-world
// "Parameter verification failed." replies set both, and the map is what
// actually says which parameter was wrong.
func TestClientAPIErrorIncludesFieldErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "Parameter verification failed.\n",
			"errors": map[string]string{
				"rootfs": "format error - unable to parse volume ID",
				"vmid":   "already used",
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret123", true)

	_, err := client.Version(context.Background())
	if err == nil {
		t.Fatal("Version() error = nil, want error")
	}
	want := "Parameter verification failed. (rootfs: format error - unable to parse volume ID; vmid: already used)"
	if err.Error() != want {
		t.Errorf("Version() error = %q, want %q", err.Error(), want)
	}
}

// TestClientAPIErrorFieldErrorsOnly confirms the field-errors-only case
// (no Message) doesn't gain a stray leading "()" or similar artifact.
func TestClientAPIErrorFieldErrorsOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": map[string]string{"hostname": "value does not look like a valid hostname"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret123", true)

	_, err := client.Version(context.Background())
	if err == nil {
		t.Fatal("Version() error = nil, want error")
	}
	want := "hostname: value does not look like a valid hostname"
	if err.Error() != want {
		t.Errorf("Version() error = %q, want %q", err.Error(), want)
	}
}

func TestClientTLSCertificateErrorHint(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"version": "8.2.1"}})
	}))
	defer server.Close()

	// insecureSkipVerify=false: server's self-signed cert will be rejected.
	client := NewClient(server.URL, "user@pve!test", "secret123", false)

	_, err := client.Version(context.Background())
	if err == nil {
		t.Fatal("Version() error = nil, want TLS verification error")
	}
	if !stringsContains(err.Error(), "insecure-skip-verify") {
		t.Errorf("Version() error = %q, want it to mention --insecure-skip-verify", err.Error())
	}
}

func TestClientDebugLoggingDisabledByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"version": "8.2.1"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret123", true)
	var buf bytes.Buffer
	client.debugOut = &buf

	if _, err := client.Version(context.Background()); err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("debug output = %q, want empty when debug logging is disabled", buf.String())
	}
}

func TestClientDebugLoggingEnabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"version": "8.2.1"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret123", true)
	var buf bytes.Buffer
	client.debugOut = &buf
	client.SetDebug(true)

	if _, err := client.Version(context.Background()); err != nil {
		t.Fatalf("Version() error = %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "GET") || !strings.Contains(out, "/api2/json/version") {
		t.Errorf("debug output = %q, want it to mention the GET request and its path", out)
	}
	if !strings.Contains(out, "200") {
		t.Errorf("debug output = %q, want it to mention the 200 response status", out)
	}
	if strings.Contains(out, "secret123") {
		t.Errorf("debug output = %q, must never contain the token secret", out)
	}
}

func TestNewClientTrimsTrailingSlashFromHost(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"version": "8.2.1"}})
	}))
	defer server.Close()

	// Trailing slash(es) on the host must not produce a doubled "//" in
	// the request path.
	client := NewClient(server.URL+"/", "user@pve!test", "secret123", true)

	if _, err := client.Version(context.Background()); err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if gotPath != "/api2/json/version" {
		t.Errorf("request path = %q, want /api2/json/version (no doubled slash)", gotPath)
	}
}

func TestNewClientSetsRequestTimeout(t *testing.T) {
	client := NewClient("https://pve.example.com:8006", "user@pve!test", "secret123", true)

	if client.httpClient.Timeout != requestTimeout {
		t.Errorf("httpClient.Timeout = %v, want %v", client.httpClient.Timeout, requestTimeout)
	}
}

func TestClientTimesOutOnUnresponsiveServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret123", true)
	client.httpClient.Timeout = 50 * time.Millisecond

	start := time.Now()
	_, err := client.Version(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Version() error = nil, want a timeout error")
	}
	if elapsed > 1*time.Second {
		t.Errorf("Version() took %v to return, want well under the server's 2s delay (client should time out first)", elapsed)
	}
	if !stringsContains(err.Error(), "deadline exceeded") {
		t.Errorf("Version() error = %q, want it to mention a timeout (deadline exceeded)", err.Error())
	}
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
