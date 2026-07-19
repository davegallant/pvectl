package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/coder/websocket"
)

// ConsoleKind selects which Proxmox API prefix ("lxc" or "qemu") a console
// call targets — the termproxy/vncwebsocket endpoints are identical in
// shape between the two, differing only in this path segment.
type ConsoleKind string

const (
	KindContainer ConsoleKind = "lxc"
	KindVM        ConsoleKind = "qemu"
)

type consoleTicket struct {
	Ticket string    `json:"ticket"`
	Port   loosePort `json:"port"`
}

// loosePort decodes a JSON number or string into a string. Proxmox's API
// viewer types termproxy's "port" field as an integer, but other
// numeric-looking fields have been observed sent as JSON strings on
// different endpoints instead (see looseInt64's precedent in backup.go),
// and the actual wire type here hasn't been confirmed against a live
// cluster from this sandbox — tolerant decoding avoids failing the whole
// console connection if that assumption turns out wrong.
type loosePort string

func (p *loosePort) UnmarshalJSON(b []byte) error {
	*p = loosePort(strings.Trim(string(b), `"`))
	return nil
}

type termproxyResponse struct {
	Data consoleTicket `json:"data"`
}

// termproxy POSTs /nodes/{node}/{lxc,qemu}/{vmid}/termproxy, which spawns
// a termproxy process on the node and returns a single-use vncticket +
// port for the websocket connection OpenConsole opens next. Requires
// VM.Console permission on the token; on LXC this has always worked with
// API token auth (LXC.pm's termproxy already passes --vncticket-endpoint
// to termproxy), and on QEMU it works as of qemu-server >= 9.1.7 (Proxmox
// bug 6079 — earlier versions only accepted a user ticket/cookie here).
func (c *Client) termproxy(ctx context.Context, node string, vmid int, kind ConsoleKind) (consoleTicket, error) {
	path := fmt.Sprintf("/nodes/%s/%s/%d/termproxy", node, kind, vmid)
	var resp termproxyResponse
	if err := c.do(ctx, http.MethodPost, path, nil, &resp); err != nil {
		return consoleTicket{}, err
	}
	return resp.Data, nil
}

// consoleWebsocketURL builds the wss:// (or ws://, matching baseURL's
// scheme) URL for the console websocket, carrying port/vncticket as query
// parameters the way Proxmox's own web UI does.
func (c *Client) consoleWebsocketURL(node string, vmid int, kind ConsoleKind, ticket consoleTicket) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parsing base url: %w", err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("unsupported scheme %q for console websocket", u.Scheme)
	}
	u.Path = path.Join(u.Path, "nodes", node, string(kind), strconv.Itoa(vmid), "vncwebsocket")
	q := u.Query()
	q.Set("port", string(ticket.Port))
	q.Set("vncticket", ticket.Ticket)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// OpenConsole requests a termproxy ticket for vmid on node, opens the
// console websocket, and completes termproxy's own auth handshake,
// returning a connection internal/term can immediately start speaking
// Proxmox's termproxy framing protocol (0:LEN:MSG / 1:COLS:ROWS: / 2)
// over. The websocket handshake uses a client built from the same
// Transport as c's normal REST client (so it shares the
// InsecureSkipVerify TLS setting) but without c's httpClient's 30s
// requestTimeout: http.Client.Timeout keeps interrupting reads on the
// response body even after a successful upgrade, which would silently
// kill any console session longer than 30 seconds.
//
// The query-param vncticket only authenticates the websocket connection
// itself (pveproxy's side); termproxy — the separate process actually
// driving the console — requires its own first-message handshake of
// "user:ticket\n" before it'll accept any terminal data, exactly as
// Proxmox's own web console sends via `AttachAddon`/`console.js`. Skipping
// this leaves termproxy waiting forever on session input it'll never get.
func (c *Client) OpenConsole(ctx context.Context, node string, vmid int, kind ConsoleKind) (*websocket.Conn, error) {
	ticket, err := c.termproxy(ctx, node, vmid, kind)
	if err != nil {
		return nil, fmt.Errorf("requesting console ticket: %w", err)
	}

	wsURL, err := c.consoleWebsocketURL(node, vmid, kind, ticket)
	if err != nil {
		return nil, err
	}

	dialClient := &http.Client{Transport: c.httpClient.Transport}
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPClient: dialClient,
		HTTPHeader: http.Header{
			"Authorization": {fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret)},
		},
		// Matches the subprotocol Proxmox's own web console negotiates
		// (`new WebSocket(url, "binary")` in console.js/AttachAddon) —
		// unconfirmed whether pveproxy requires it, but it's free to match.
		Subprotocols: []string{"binary"},
	})
	if err != nil {
		return nil, fmt.Errorf("opening console websocket: %w", err)
	}

	// The handshake's user must match whatever Proxmox assembled the
	// ticket against server-side ($authuser in API2/{LXC,Qemu}.pm, which
	// for a token-authenticated request is the full "user@realm!token"
	// string) — c.tokenID, matching bug 6079's rejection message format
	// ('user@realm!token' does not look like a valid user name). Unverified
	// against a live cluster: if the handshake is rejected, the first
	// thing to try is the bare "user@realm" with the "!token" suffix
	// stripped.
	handshake := fmt.Sprintf("%s:%s\n", c.tokenID, ticket.Ticket)
	if err := conn.Write(ctx, websocket.MessageText, []byte(handshake)); err != nil {
		_ = conn.CloseNow()
		return nil, fmt.Errorf("sending console auth handshake: %w", err)
	}

	return conn, nil
}
