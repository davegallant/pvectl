package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// NodeStorage is one storage's configuration entry from
// GET /nodes/{node}/storage — distinct from StorageResource
// (/cluster/resources), which carries usage stats but not which content
// types a storage accepts.
type NodeStorage struct {
	Storage string
	// Content is the storage's configured content types (e.g. "rootdir",
	// "images", "vztmpl", "backup", "iso") — what Proxmox's own storage
	// selector filters on for a given operation.
	Content []string
}

// SupportsContent reports whether the storage accepts the given content
// type (e.g. "rootdir" for an LXC container's root filesystem).
func (s NodeStorage) SupportsContent(contentType string) bool {
	for _, c := range s.Content {
		if c == contentType {
			return true
		}
	}
	return false
}

type nodeStorageEntry struct {
	Storage string `json:"storage"`
	Content string `json:"content"`
}

type nodeStorageResponse struct {
	Data []nodeStorageEntry `json:"data"`
}

// ListNodeStorages fetches every storage configured on node along with
// its accepted content types.
func (c *Client) ListNodeStorages(ctx context.Context, node string) ([]NodeStorage, error) {
	var resp nodeStorageResponse
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/storage", node), nil, &resp); err != nil {
		return nil, err
	}

	storages := make([]NodeStorage, len(resp.Data))
	for i, e := range resp.Data {
		storages[i] = NodeStorage{Storage: e.Storage, Content: strings.Split(e.Content, ",")}
	}
	return storages, nil
}
