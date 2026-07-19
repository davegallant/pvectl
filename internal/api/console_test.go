package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConsoleWebsocketURL(t *testing.T) {
	tests := []struct {
		name string
		host string
		kind ConsoleKind
		want string
	}{
		{
			name: "https becomes wss",
			host: "https://pve.example.com:8006",
			kind: KindContainer,
			want: "wss://pve.example.com:8006/api2/json/nodes/pve1/lxc/101/vncwebsocket?port=1234&vncticket=tick%2Fet",
		},
		{
			name: "http becomes ws",
			host: "http://pve.example.com:8006",
			kind: KindVM,
			want: "ws://pve.example.com:8006/api2/json/nodes/pve1/qemu/101/vncwebsocket?port=1234&vncticket=tick%2Fet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(tt.host, "user@pve!test", "secret", true)
			got, err := c.consoleWebsocketURL("pve1", 101, tt.kind, consoleTicket{Ticket: "tick/et", Port: "1234"})
			if err != nil {
				t.Fatalf("consoleWebsocketURL() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("consoleWebsocketURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientTermproxy(t *testing.T) {
	tests := []struct {
		name     string
		portJSON string // raw JSON for the "port" field, exercising loosePort
	}{
		{"port as JSON string", `"5900"`},
		{"port as JSON number", `5900`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api2/json/nodes/pve1/lxc/101/termproxy" {
					t.Errorf("request path = %q, want /api2/json/nodes/pve1/lxc/101/termproxy", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("request method = %q, want POST", r.Method)
				}
				fmt.Fprintf(w, `{"data":{"ticket":"PVEVNC:abc123","port":%s}}`, tt.portJSON)
			}))
			defer server.Close()

			client := NewClient(server.URL, "user@pve!test", "secret123", true)

			got, err := client.termproxy(context.Background(), "pve1", 101, KindContainer)
			if err != nil {
				t.Fatalf("termproxy() error = %v", err)
			}
			if got.Ticket != "PVEVNC:abc123" || got.Port != "5900" {
				t.Errorf("termproxy() = %+v, want ticket=PVEVNC:abc123 port=5900", got)
			}
		})
	}
}

func TestClientTermproxyQemuPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve1/qemu/202/termproxy" {
			t.Errorf("request path = %q, want /api2/json/nodes/pve1/qemu/202/termproxy", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]string{"ticket": "PVEVNC:xyz", "port": "5901"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret123", true)

	if _, err := client.termproxy(context.Background(), "pve1", 202, KindVM); err != nil {
		t.Fatalf("termproxy() error = %v", err)
	}
}
