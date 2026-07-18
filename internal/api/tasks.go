package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// TaskStatus is a Proxmox task's state, as returned by
// GET /nodes/{node}/tasks/{upid}/status.
type TaskStatus struct {
	Status     string `json:"status"`     // "running" or "stopped"
	ExitStatus string `json:"exitstatus"` // meaningful once Status != "running"
}

// Done reports whether the task has finished (successfully or not).
func (s TaskStatus) Done() bool { return s.Status != "running" }

// TaskCompletedWithWarnings reports whether exitStatus is Proxmox's
// non-fatal "WARNINGS: N" form (e.g. a container create that emitted a
// systemd-nesting hint, or an LVM thin-pool overcommit notice) — a task
// that finished with caveats worth surfacing, not a failure the way any
// other non-"OK" exit status is. Proxmox's own GUI likewise shows this as
// a completed task with a warning icon, not a failed one. Shared by
// TaskStatus.Failed() and cmd/tasks.go's taskStatusLabel so the two
// "is this really a failure" checks can't drift apart.
func TaskCompletedWithWarnings(exitStatus string) bool {
	return strings.HasPrefix(exitStatus, "WARNINGS:")
}

// Failed reports whether a finished task did not exit cleanly — anything
// other than "OK", except TaskCompletedWithWarnings' non-fatal
// "WARNINGS: N" form. Meaningless while the task is still running —
// callers should check Done first.
func (s TaskStatus) Failed() bool {
	return s.Done() && s.ExitStatus != "OK" && !TaskCompletedWithWarnings(s.ExitStatus)
}

// TaskStatus fetches a single point-in-time read of upid's status on node.
func (c *Client) TaskStatus(ctx context.Context, node, upid string) (TaskStatus, error) {
	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", node, upid)
	var resp struct {
		Data TaskStatus `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return TaskStatus{}, err
	}
	return resp.Data, nil
}

// TaskLogLine is one line of a task's log, as returned by
// GET /nodes/{node}/tasks/{upid}/log. N is the line's 1-based sequence
// number; T is its text.
type TaskLogLine struct {
	N int    `json:"n"`
	T string `json:"t"`
}

// TaskLog fetches upid's full log on node. Line contents are Proxmox's
// own free-form, version-dependent text — callers should display it
// as-is, not parse it, unless the format has been verified against a
// real capture.
func (c *Client) TaskLog(ctx context.Context, node, upid string) ([]TaskLogLine, error) {
	path := fmt.Sprintf("/nodes/%s/tasks/%s/log", node, upid)
	var resp struct {
		Data []TaskLogLine `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// ClusterTask is one entry from GET /cluster/tasks — a single worker
// task recorded anywhere in the cluster, running or finished. Status
// and EndTime are the JSON zero value while the task is still running;
// Type and ID are Proxmox's own stable task-registry strings (e.g.
// "qmigrate"/"101"), unlike TaskLogLine's free-form log text.
type ClusterTask struct {
	UPID      string `json:"upid"`
	Node      string `json:"node"`
	User      string `json:"user"`
	Type      string `json:"type"`
	ID        string `json:"id"`
	StartTime int64  `json:"starttime"`
	EndTime   int64  `json:"endtime"`
	Status    string `json:"status"`
}

// Running reports whether the task has not finished yet.
func (t ClusterTask) Running() bool { return t.EndTime == 0 }

// ClusterTasks fetches the cluster's recent tasks (Proxmox's own default
// recency window — no client-side filtering or pagination).
func (c *Client) ClusterTasks(ctx context.Context) ([]ClusterTask, error) {
	var resp struct {
		Data []ClusterTask `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/cluster/tasks", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
