package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func captureTaskOutput(t *testing.T, run func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = previous }()
	runErr := run()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String(), runErr
}

func TestTaskUPIDNodeRejectsMalformedIDs(t *testing.T) {
	for _, upid := range []string{"", "pve1", "UPID::123", "UPID:pve1:", "UPID:..:123", "UPID:pve1:123/../other", "UPID:pve1:123?foo"} {
		if _, err := taskUPIDNode(upid); err == nil {
			t.Errorf("taskUPIDNode(%q) accepted malformed UPID", upid)
		}
	}
	if node, err := taskUPIDNode("UPID:pve1:1234:root@pam:"); err != nil || node != "pve1" {
		t.Errorf("taskUPIDNode(valid) = (%q, %v), want pve1", node, err)
	}
}

func TestRunTaskStatusPrintsActualStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes/pve1/tasks/UPID:pve1:1234/status" {
			t.Errorf("status path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "WARNINGS: 1"}})
	}))
	defer server.Close()
	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	output, err := captureTaskOutput(t, func() error { return runTaskStatus(client, "UPID:pve1:1234") })
	if err != nil || !strings.Contains(output, "warning: WARNINGS: 1") {
		t.Errorf("runTaskStatus() = (%q, %v), want warning status", output, err)
	}
}

func TestRunTaskLogsPrintsLines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"n": 1, "t": "first"}, {"n": 2, "t": "second"}}, "total": 2})
	}))
	defer server.Close()
	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	output, err := captureTaskOutput(t, func() error { return runTaskLogs(client, "UPID:pve1:1234") })
	if err != nil || output != "first\nsecond\n" {
		t.Errorf("runTaskLogs() = (%q, %v), want full log text", output, err)
	}
}

func TestRunTaskWaitReturnsFailedOutcome(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/status"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "disk full"}})
		case strings.HasSuffix(r.URL.Path, "/log"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"n": 1, "t": "ERROR: disk full"}}, "total": 1})
		default:
			t.Errorf("unexpected task path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	output, err := captureTaskOutput(t, func() error { return runTaskWait(client, "UPID:pve1:1234") })
	if err == nil || !strings.Contains(output, "ERROR: disk full") {
		t.Errorf("runTaskWait() = (%q, %v), want failed outcome and log", output, err)
	}
}

func TestTaskDetailCommandsAreRegistered(t *testing.T) {
	for _, name := range []string{"status", "logs", "wait"} {
		cmd, _, err := rootCmd.Find([]string{"tasks", name})
		if err != nil || cmd.Name() != name {
			t.Errorf("tasks %s lookup = (%v, %v)", name, cmd, err)
		}
	}
}
