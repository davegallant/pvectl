package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientTaskLog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve1/tasks/UPID:pve1:1234/log" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"n": 1, "t": "task started"},
			{"n": 2, "t": "migration finished"},
		}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	lines, err := client.TaskLog(context.Background(), "pve1", "UPID:pve1:1234")
	if err != nil {
		t.Fatalf("TaskLog() error = %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want 2", len(lines))
	}
	if lines[0].N != 1 || lines[0].T != "task started" {
		t.Errorf("lines[0] = %+v, want {N:1 T:\"task started\"}", lines[0])
	}
	if lines[1].N != 2 || lines[1].T != "migration finished" {
		t.Errorf("lines[1] = %+v, want {N:2 T:\"migration finished\"}", lines[1])
	}
}

func TestClientTaskStatusRunning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve1/tasks/UPID:pve1:1234/status" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "running"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	status, err := client.TaskStatus(context.Background(), "pve1", "UPID:pve1:1234")
	if err != nil {
		t.Fatalf("TaskStatus() error = %v", err)
	}
	if status.Done() {
		t.Error("Done() = true for a running task, want false")
	}
	if status.Failed() {
		t.Error("Failed() = true for a running task, want false")
	}
}

func TestClientTaskStatusStoppedOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	status, err := client.TaskStatus(context.Background(), "pve1", "UPID:pve1:1234")
	if err != nil {
		t.Fatalf("TaskStatus() error = %v", err)
	}
	if !status.Done() {
		t.Error("Done() = false for a stopped task, want true")
	}
	if status.Failed() {
		t.Error("Failed() = true for an OK exit status, want false")
	}
}

func TestClientTaskStatusStoppedFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "unable to open file"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	status, err := client.TaskStatus(context.Background(), "pve1", "UPID:pve1:1234")
	if err != nil {
		t.Fatalf("TaskStatus() error = %v", err)
	}
	if !status.Done() {
		t.Error("Done() = false for a stopped task, want true")
	}
	if !status.Failed() {
		t.Error("Failed() = false for a non-OK exit status, want true")
	}
}

// TestClientTaskStatusStoppedWarningsNotFailed confirms Proxmox's
// non-fatal "WARNINGS: N" exit status (e.g. a systemd-nesting hint on an
// otherwise fully-created container) is not treated as a failure, unlike
// any other non-"OK" exit status — matching Proxmox's own GUI, which
// shows this as a completed task with a warning icon, not a failed one.
func TestClientTaskStatusStoppedWarningsNotFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "WARNINGS: 1"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	status, err := client.TaskStatus(context.Background(), "pve1", "UPID:pve1:1234")
	if err != nil {
		t.Fatalf("TaskStatus() error = %v", err)
	}
	if !status.Done() {
		t.Error("Done() = false for a stopped task, want true")
	}
	if status.Failed() {
		t.Error("Failed() = true for a WARNINGS-only exit status, want false")
	}
}

func TestTaskCompletedWithWarnings(t *testing.T) {
	tests := []struct {
		exitStatus string
		want       bool
	}{
		{"OK", false},
		{"WARNINGS: 1", true},
		{"WARNINGS: 3", true},
		{"unable to open file", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := TaskCompletedWithWarnings(tt.exitStatus); got != tt.want {
			t.Errorf("TaskCompletedWithWarnings(%q) = %v, want %v", tt.exitStatus, got, tt.want)
		}
	}
}

func TestClientClusterTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/cluster/tasks" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{
				"upid": "UPID:pve1:1234:...", "node": "pve1", "user": "root@pam",
				"type": "qmigrate", "id": "101", "starttime": 1000, "endtime": 1010,
				"status": "OK",
			},
			{
				"upid": "UPID:pve1:5678:...", "node": "pve1", "user": "root@pam",
				"type": "vzstart", "id": "201", "starttime": 2000,
			},
		}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@pve!test", "secret", true)
	tasks, err := client.ClusterTasks(context.Background())
	if err != nil {
		t.Fatalf("ClusterTasks() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("len(tasks) = %d, want 2", len(tasks))
	}
	if tasks[0].Running() {
		t.Error("tasks[0].Running() = true for a task with an endtime, want false")
	}
	if !tasks[1].Running() {
		t.Error("tasks[1].Running() = false for a task with no endtime, want true")
	}
}
