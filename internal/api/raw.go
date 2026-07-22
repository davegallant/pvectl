package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// RawRequest issues an arbitrary Proxmox API call to path (e.g.
// "/nodes/pve1/status/current") with the given HTTP method, returning the
// raw JSON response exactly as Proxmox sent it — envelope included
// (typically `{"data": ...}`) — for `pvectl api` and any other caller
// that needs to reach an endpoint pvectl has no dedicated method for.
// params is form-encoded into the request body for POST/PUT/DELETE, or
// appended to path's query string for GET, matching where Proxmox's own
// API expects each method's parameters.
func (c *Client) RawRequest(ctx context.Context, method, path string, params url.Values) (json.RawMessage, error) {
	var body io.Reader
	if len(params) > 0 {
		if method == http.MethodGet {
			sep := "?"
			if strings.Contains(path, "?") {
				sep = "&"
			}
			path += sep + params.Encode()
		} else {
			body = strings.NewReader(params.Encode())
		}
	}

	var raw json.RawMessage
	if err := c.do(ctx, method, path, body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
