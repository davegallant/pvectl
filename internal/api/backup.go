package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Backup triggers a vzdump backup of a single guest (LXC container or QEMU
// VM — the vzdump endpoint is node-scoped and doesn't distinguish guest
// type) to the given storage, returning the Proxmox task ID (UPID) so the
// caller can reference it (e.g. in the Proxmox UI's Task History or via
// GET /nodes/{node}/tasks/{upid}/status). Like Start/Stop/Reboot, pvectl
// doesn't poll the task to completion — vzdump runs in the background.
func (c *Client) Backup(ctx context.Context, node string, vmid int, storage string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/vzdump", node)
	form := url.Values{"vmid": {strconv.Itoa(vmid)}}
	if storage != "" {
		form.Set("storage", storage)
	}
	return c.postUPID(ctx, path, strings.NewReader(form.Encode()))
}

// Backup is one vzdump archive found on a storage's content listing.
type Backup struct {
	VolID   string `json:"volid"`
	Storage string `json:"storage"`
	Node    string `json:"node"`
	VMID    int    `json:"vmid"`
	CTime   int64  `json:"ctime"`
	Size    int64  `json:"size"`
	Format  string `json:"format"`
	Notes   string `json:"notes"`
}

// looseInt64 decodes a JSON number or a JSON string containing a number
// into an int64. Proxmox's storage content listing has been observed
// returning numeric fields (ctime, size, vmid) as JSON strings for some
// storage types (e.g. LVM-thin) instead of numbers — this makes decoding
// tolerant of either instead of failing the whole listing.
type looseInt64 int64

func (n *looseInt64) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*n = looseInt64(v)
	return nil
}

type storageContentEntry struct {
	VolID   string     `json:"volid"`
	Content string     `json:"content"`
	CTime   looseInt64 `json:"ctime"`
	Size    looseInt64 `json:"size"`
	Format  string     `json:"format"`
	VMID    looseInt64 `json:"vmid"`
	Notes   string     `json:"notes"`
}

type storageContentResponse struct {
	Data []storageContentEntry `json:"data"`
}

// ListBackups returns every vzdump backup archive for vmid found across
// storages (storage IDs mounted on node), newest first. Filters
// client-side on content=="backup" and vmid rather than relying on the
// content endpoint's own query-param filters — /cluster/resources's
// server-side type filter has already proven unreliable on a real
// cluster (see AGENTS.md), so this sticks to the same fetch-then-filter
// pattern as ListContainers/ListVMs.
func (c *Client) ListBackups(ctx context.Context, node string, storages []string, vmid int) ([]Backup, error) {
	return c.listBackups(ctx, node, storages, &vmid)
}

// ListAllBackups returns every vzdump backup archive found across
// storages (storage IDs mounted on node), for any vmid, newest first.
// Unlike ListBackups it isn't scoped to one guest — used for disaster
// recovery, where the original guest may no longer exist to filter by.
func (c *Client) ListAllBackups(ctx context.Context, node string, storages []string) ([]Backup, error) {
	return c.listBackups(ctx, node, storages, nil)
}

// listBackups is ListBackups/ListAllBackups's shared implementation.
// vmidFilter nil means "every vmid" (ListAllBackups); non-nil restricts
// to that one vmid (ListBackups).
//
// The per-storage GET /nodes/{node}/storage/{storage}/content calls are
// mutually independent, so they fan out concurrently: N storages now cost
// ~1 round trip instead of N sequential ones (the win scales with the
// node's storage count). The underlying *http.Client/Transport are
// documented concurrency-safe, so a shared *Client is fine across the
// goroutines. Results are kept per-goroutine in a slice indexed by the
// storage's position so the final merge is deterministic, and the first
// error (lowest storage index) is returned first — matching the original
// sequential version's in-order error reporting.
func (c *Client) listBackups(ctx context.Context, node string, storages []string, vmidFilter *int) ([]Backup, error) {
	type result struct {
		backups []Backup
		err     error
	}
	results := make([]result, len(storages))
	var wg sync.WaitGroup
	for i, storage := range storages {
		wg.Add(1)
		go func(i int, storage string) {
			defer wg.Done()
			path := fmt.Sprintf("/nodes/%s/storage/%s/content", node, storage)
			var resp storageContentResponse
			if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
				results[i].err = fmt.Errorf("listing content on %s/%s: %w", node, storage, err)
				return
			}
			for _, e := range resp.Data {
				if e.Content != "backup" {
					continue
				}
				if vmidFilter != nil && int(e.VMID) != *vmidFilter {
					continue
				}
				results[i].backups = append(results[i].backups, Backup{
					VolID:   e.VolID,
					Storage: storage,
					Node:    node,
					VMID:    int(e.VMID),
					CTime:   int64(e.CTime),
					Size:    int64(e.Size),
					Format:  e.Format,
					Notes:   e.Notes,
				})
			}
		}(i, storage)
	}
	wg.Wait()

	// Merge in storage-index order so the first error reported is the same
	// one the sequential version would have surfaced first.
	var backups []Backup
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		backups = append(backups, r.backups...)
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CTime > backups[j].CTime
	})
	return backups, nil
}

// DeleteBackup permanently deletes one vzdump archive (volid, as returned
// by ListBackups) from storage on node. Proxmox does not move deleted
// archives to a trash/recycle bin — there is no undo.
func (c *Client) DeleteBackup(ctx context.Context, node, storage, volid string) error {
	// volid (e.g. "local:backup/vzdump-lxc-101-....tar.zst") contains a
	// "/", but the API expects it as a single opaque path segment —
	// PathEscape percent-encodes "/" (unlike url.QueryEscape's "+"
	// space-encoding, which would also be wrong here).
	path := fmt.Sprintf("/nodes/%s/storage/%s/content/%s", node, storage, url.PathEscape(volid))
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}
