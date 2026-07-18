package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// VM is a QEMU virtual machine as returned by /cluster/resources.
type VM struct {
	VMID   int
	Name   string
	Node   string
	Status string
}

// ListVMs returns every QEMU VM across the cluster, sorted by VMID
// ascending — mirrors ListContainers's ordering guarantee (and reason:
// /cluster/resources doesn't guarantee any particular order).
func (c *Client) ListVMs(ctx context.Context) ([]VM, error) {
	entries, err := c.fetchResources(ctx)
	if err != nil {
		return nil, err
	}

	var vms []VM
	for _, r := range entries {
		if r.Type != "qemu" {
			continue
		}
		vms = append(vms, VM{
			VMID:   r.VMID,
			Name:   r.Name,
			Node:   r.Node,
			Status: r.Status,
		})
	}
	sort.Slice(vms, func(i, j int) bool {
		return vms[i].VMID < vms[j].VMID
	})
	return vms, nil
}

// StartVM, StopVM, RebootVM, and SnapshotVM return the Proxmox task UPID
// for the triggered action so callers can poll TaskStatus for completion,
// instead of only firing the request and returning — mirrors the LXC side
// (lxc.go's Start/Stop/Reboot/Snapshot) exactly.

func (c *Client) StartVM(ctx context.Context, node string, vmid int) (string, error) {
	return c.statusVMAction(ctx, node, vmid, "start")
}

func (c *Client) StopVM(ctx context.Context, node string, vmid int) (string, error) {
	return c.statusVMAction(ctx, node, vmid, "shutdown")
}

func (c *Client) RebootVM(ctx context.Context, node string, vmid int) (string, error) {
	return c.statusVMAction(ctx, node, vmid, "reboot")
}

func (c *Client) statusVMAction(ctx context.Context, node string, vmid int, action string) (string, error) {
	return c.postUPID(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/%s", node, vmid, action), nil)
}

// MigrateVM moves a VM from node to target, returning the Proxmox task
// UPID. online should be true when the VM is currently running — Proxmox
// performs a true live migration (memory copied while the guest keeps
// running), unlike Migrate's LXC restart-based approach. A stopped VM
// needs no online flag; Proxmox just relocates its config/disks.
func (c *Client) MigrateVM(ctx context.Context, node string, vmid int, target string, online bool) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/migrate", node, vmid)
	form := url.Values{"target": {target}}
	if online {
		form.Set("online", "1")
	}
	return c.postUPID(ctx, path, strings.NewReader(form.Encode()))
}

func (c *Client) SnapshotVM(ctx context.Context, node string, vmid int, name string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", node, vmid)
	form := url.Values{"snapname": {name}}
	return c.postUPID(ctx, path, strings.NewReader(form.Encode()))
}

// DeleteVM permanently destroys a VM, returning the Proxmox task UPID —
// mirrors DeleteContainer, except there's no force param: Proxmox's own
// DELETE .../qemu/{vmid} has no equivalent to the LXC endpoint's `force`
// (destroy while running); a running VM must be stopped first, same as
// an LXC container without ct delete's --force. purge removes the VMID
// from backup jobs, replication jobs, HA, and ACLs; without it those
// references are left dangling (matching Proxmox's own default).
func (c *Client) DeleteVM(ctx context.Context, node string, vmid int, purge bool) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d", node, vmid)
	if purge {
		path += "?" + url.Values{"purge": {"1"}}.Encode()
	}
	var resp struct {
		Data string `json:"data"`
	}
	if err := c.do(ctx, http.MethodDelete, path, nil, &resp); err != nil {
		return "", err
	}
	return resp.Data, nil
}

// VMConfig is a QEMU VM's full configuration, as returned by
// GET /nodes/{node}/qemu/{vmid}/config. Unlike the LXC Config, there is no
// RawLXC-equivalent passthrough block — QEMU config keys are flat, so
// every non-digest key fits in Fields.
type VMConfig struct {
	Digest string
	Fields map[string]string
}

func (c *Client) GetVMConfig(ctx context.Context, node string, vmid int) (VMConfig, error) {
	var resp struct {
		Data map[string]any `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid), nil, &resp); err != nil {
		return VMConfig{}, err
	}
	digest, fields, _ := configFromData(resp.Data, false)
	return VMConfig{Digest: digest, Fields: fields}, nil
}

func (c *Client) PutVMConfig(ctx context.Context, node string, vmid int, changed map[string]string, digest string) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid)
	form := url.Values{"digest": {digest}}
	for k, v := range changed {
		form.Set(k, v)
	}
	return classifyPutError(c.do(ctx, http.MethodPut, path, strings.NewReader(form.Encode()), nil))
}
