package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestClientRawRequestGETUsesQueryString(t *testing.T) {
	var gotPath, gotQuery, gotMethod, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []string{"pve1", "pve2"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	raw, err := client.RawRequest(context.Background(), http.MethodGet, "/nodes", url.Values{"full": {"1"}})
	if err != nil {
		t.Fatalf("RawRequest() error = %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api2/json/nodes" {
		t.Errorf("path = %q, want /api2/json/nodes", gotPath)
	}
	if gotQuery != "full=1" {
		t.Errorf("query = %q, want full=1", gotQuery)
	}
	if gotBody != "" {
		t.Errorf("body = %q, want empty for a GET", gotBody)
	}
	if string(raw) != `{"data":["pve1","pve2"]}` {
		t.Errorf("raw = %s, want the response body verbatim", raw)
	}
}

func TestClientRawRequestPOSTUsesFormBody(t *testing.T) {
	var gotMethod, gotBody, gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": "UPID:..."})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	raw, err := client.RawRequest(context.Background(), http.MethodPost, "/nodes/pve1/lxc/101/status/start", url.Values{"vmid": {"101"}})
	if err != nil {
		t.Fatalf("RawRequest() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotBody != "vmid=101" {
		t.Errorf("body = %q, want vmid=101", gotBody)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Errorf("content-type = %q, want application/x-www-form-urlencoded", gotContentType)
	}
	if string(raw) != `{"data":"UPID:..."}` {
		t.Errorf("raw = %s, want the response body verbatim", raw)
	}
}

func TestClientRawRequestEmptyResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	raw, err := client.RawRequest(context.Background(), http.MethodDelete, "/nodes/pve1/lxc/101/firewall/rules/0", nil)
	if err != nil {
		t.Fatalf("RawRequest() error = %v, want nil for an empty response body", err)
	}
	if raw != nil {
		t.Errorf("raw = %q, want nil for an empty response body", raw)
	}
}

func TestClientRawRequestErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": nil, "message": "Permission check failed"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	_, err := client.RawRequest(context.Background(), http.MethodGet, "/access/users", nil)
	if err == nil {
		t.Fatal("RawRequest() error = nil, want an error for a 403 response")
	}
	if err.Error() != "Permission check failed" {
		t.Errorf("err = %q, want %q", err.Error(), "Permission check failed")
	}
}
