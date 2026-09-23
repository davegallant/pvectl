package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/davegallant/pvectl/internal/api"
)

func TestTaskDetailsUseCommandCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	old := rootCmd.Context()
	rootCmd.SetContext(ctx)
	t.Cleanup(func() { rootCmd.SetContext(old) })
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled task detail request reached server")
	}))
	defer s.Close()
	client := api.NewClient(s.URL, "user@pve!test", "secret", true)
	for _, run := range []func(*api.Client, string) error{runTaskStatus, runTaskLogs} {
		if err := run(client, "UPID:pve1:1234"); !errors.Is(err, context.Canceled) {
			t.Errorf("task detail error = %v, want context cancellation", err)
		}
	}
}

func TestTaskLogsCancelInFlightFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	old := rootCmd.Context()
	rootCmd.SetContext(ctx)
	t.Cleanup(func() { rootCmd.SetContext(old) })
	started := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer s.Close()
	client := api.NewClient(s.URL, "user@pve!test", "secret", true)
	done := make(chan error, 1)
	go func() { done <- runTaskLogs(client, "UPID:pve1:1234") }()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("runTaskLogs() error = %v, want cancellation", err)
		}
	case <-time.After(time.Second):
		s.CloseClientConnections()
		t.Fatal("in-flight task log fetch did not stop after cancellation")
	}
}

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
