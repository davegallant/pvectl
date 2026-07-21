package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
)

// Snapshot is one guest snapshot, as returned by GET
// /nodes/{node}/lxc|qemu/{vmid}/snapshot.
type Snapshot struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SnapTime    int64  `json:"snaptime"`
	Parent      string `json:"parent"`
}

type snapshotEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SnapTime    int64  `json:"snaptime"`
	Parent      string `json:"parent"`
}

type snapshotListResponse struct {
	Data []snapshotEntry `json:"data"`
}

// toSnapshots converts entries to Snapshots, dropping Proxmox's synthetic
// "current" entry (the live guest state, always included in the listing
// alongside real snapshots — it has no snaptime and can't be deleted, so
// showing it in a snapshot listing would be misleading), and sorts the
// rest newest first, matching ListBackups's convention.
func toSnapshots(entries []snapshotEntry) []Snapshot {
	var snapshots []Snapshot
	for _, e := range entries {
		if e.Name == "current" {
			continue
		}
		snapshots = append(snapshots, Snapshot(e))
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].SnapTime > snapshots[j].SnapTime
	})
	return snapshots
}

// ListSnapshots returns vmid's LXC container snapshots, newest first.
func (c *Client) ListSnapshots(ctx context.Context, node string, vmid int) ([]Snapshot, error) {
	path := fmt.Sprintf("/nodes/%s/lxc/%d/snapshot", node, vmid)
	var resp snapshotListResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return toSnapshots(resp.Data), nil
}

// ListSnapshotsVM is ListSnapshots's mirror for QEMU VMs.
func (c *Client) ListSnapshotsVM(ctx context.Context, node string, vmid int) ([]Snapshot, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", node, vmid)
	var resp snapshotListResponse
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return toSnapshots(resp.Data), nil
}

// DeleteSnapshot permanently deletes one of vmid's LXC container
// snapshots, returning the Proxmox task UPID — unlike DeleteBackup,
// snapshot removal is an async task (it may need to merge disk state),
// so callers should poll it to completion the same way Start/Stop/
// Backup/Migrate already do.
func (c *Client) DeleteSnapshot(ctx context.Context, node string, vmid int, name string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/lxc/%d/snapshot/%s", node, vmid, url.PathEscape(name))
	var resp struct {
		Data string `json:"data"`
	}
	if err := c.do(ctx, http.MethodDelete, path, nil, &resp); err != nil {
		return "", err
	}
	return resp.Data, nil
}

// DeleteSnapshotVM is DeleteSnapshot's mirror for QEMU VMs.
func (c *Client) DeleteSnapshotVM(ctx context.Context, node string, vmid int, name string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot/%s", node, vmid, url.PathEscape(name))
	var resp struct {
		Data string `json:"data"`
	}
	if err := c.do(ctx, http.MethodDelete, path, nil, &resp); err != nil {
		return "", err
	}
	return resp.Data, nil
}

// Rollback reverts vmid's LXC container to name, discarding any changes
// made since the snapshot was taken, and returns the Proxmox task UPID —
// like DeleteSnapshot, this is an async task, so callers should poll it
// to completion via runProgressAction.
func (c *Client) Rollback(ctx context.Context, node string, vmid int, name string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/lxc/%d/snapshot/%s/rollback", node, vmid, url.PathEscape(name))
	var resp struct {
		Data string `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, path, nil, &resp); err != nil {
		return "", err
	}
	return resp.Data, nil
}

// RollbackVM is Rollback's mirror for QEMU VMs.
func (c *Client) RollbackVM(ctx context.Context, node string, vmid int, name string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot/%s/rollback", node, vmid, url.PathEscape(name))
	var resp struct {
		Data string `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, path, nil, &resp); err != nil {
		return "", err
	}
	return resp.Data, nil
}
