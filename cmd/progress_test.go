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

func TestFormatProgressLine(t *testing.T) {
	got := formatProgressLine('⠋', 4*time.Second+200*time.Millisecond, "rebooting opnsense (135)")
	want := "⠋ rebooting opnsense (135)… 4s"
	if got != want {
		t.Errorf("formatProgressLine() = %q, want %q", got, want)
	}
}

func TestFormatDoneLine(t *testing.T) {
	got := formatDoneLine("rebooted opnsense (135)", 18200*time.Millisecond, false, "UPID:pve1:...")
	if !strings.HasPrefix(got, "✓ rebooted opnsense (135) (18.2s)") {
		t.Errorf("formatDoneLine() = %q, want prefix %q", got, "✓ rebooted opnsense (135) (18.2s)")
	}
	if strings.Contains(got, "UPID") {
		t.Errorf("formatDoneLine() = %q, want no UPID when verbose is false", got)
	}
}

func TestFormatDoneLineVerbose(t *testing.T) {
	got := formatDoneLine("rebooted opnsense (135)", 18200*time.Millisecond, true, "UPID:pve1:...")
	if !strings.Contains(got, "UPID:pve1:...") {
		t.Errorf("formatDoneLine() = %q, want it to contain the UPID when verbose is true", got)
	}
}

func TestFormatFailLine(t *testing.T) {
	got := formatFailLine("backed up truenas (110)", 5*time.Second, "unable to open file", false, "UPID:pve1:...")
	if !strings.HasPrefix(got, "✗ backed up truenas (110) failed after 5s: unable to open file") {
		t.Errorf("formatFailLine() = %q, want prefix mentioning the exit status", got)
	}
	if strings.Contains(got, "UPID") {
		t.Errorf("formatFailLine() = %q, want no UPID when verbose is false", got)
	}
}

func TestFormatFailLineVerbose(t *testing.T) {
	got := formatFailLine("backed up truenas (110)", 5*time.Second, "unable to open file", true, "UPID:pve1:...")
	if !strings.Contains(got, "UPID:pve1:...") {
		t.Errorf("formatFailLine() = %q, want it to contain the UPID when verbose is true", got)
	}
}

// runPollTaskQuietCapturingStdout runs pollTaskQuiet with client/args,
// capturing everything it prints to stdout, and returns both the printed
// output and pollTaskQuiet's own returned error.
func runPollTaskQuietCapturingStdout(t *testing.T, client *api.Client, node, upid, label, verbDone string) (output string, runErr error) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	runErr = pollTaskQuiet(context.Background(), client, node, upid, label, verbDone)

	os.Stdout = origStdout
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), runErr
}

// TestPollTaskQuietWarningsOnlyIsNotAFailure confirms Proxmox's non-fatal
// "WARNINGS: N" exit status (e.g. a container create that only emitted a
// systemd-nesting hint, found via a real `ct create`) is treated as a
// success — matching Proxmox's own GUI, which shows this as a completed
// task with a warning icon, not a failed one — while still printing the
// real log so the caveat isn't silently swallowed.
func TestPollTaskQuietWarningsOnlyIsNotAFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/status"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "WARNINGS: 1"}})
		case strings.HasSuffix(r.URL.Path, "/log"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"n": 1, "t": "TASK WARNINGS: 1"},
				{"n": 2, "t": "WARN: Systemd 255 detected. You may need to enable nesting."},
			}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	output, runErr := runPollTaskQuietCapturingStdout(t, client, "pve1", "UPID:pve1:...", "creating container test (144)", "created container test (144)")

	if runErr != nil {
		t.Errorf("pollTaskQuiet() error = %v, want nil for a WARNINGS-only exit status", runErr)
	}
	if !strings.Contains(output, "✓") {
		t.Errorf("output = %q, want the success (✓) line, not a failure line", output)
	}
	if !strings.Contains(output, "task log:") {
		t.Errorf("output = %q, want it to contain the task log header", output)
	}
	if !strings.Contains(output, "enable nesting") {
		t.Errorf("output = %q, want it to contain the actual task log line", output)
	}
}

// TestPollTaskQuietPrintsTaskLogOnRealFailure confirms a genuine (non-
// "WARNINGS") failure still returns an error, prints the ✗ line, and
// includes the real log — not just the bare exit status string.
func TestPollTaskQuietPrintsTaskLogOnRealFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/status"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "unable to open file"}})
		case strings.HasSuffix(r.URL.Path, "/log"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"n": 1, "t": "ERROR: unable to open '/mnt/pve/truenas-backups/dump/x.tar.zst'"},
			}})
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	output, runErr := runPollTaskQuietCapturingStdout(t, client, "pve1", "UPID:pve1:...", "backing up truenas (110)", "backed up truenas (110)")

	if runErr == nil {
		t.Fatal("pollTaskQuiet() error = nil, want an error for a real failure")
	}
	if !strings.Contains(output, "✗") {
		t.Errorf("output = %q, want the failure (✗) line", output)
	}
	if !strings.Contains(output, "task log:") {
		t.Errorf("output = %q, want it to contain the task log header", output)
	}
	if !strings.Contains(output, "unable to open") {
		t.Errorf("output = %q, want it to contain the actual task log line, not just the bare exit status", output)
	}
}

func TestInterruptedTaskWaitReturnsErrorWithUPID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := api.NewClient("http://127.0.0.1:1", "user@pve!test", "secret", true)
	const upid = "UPID:pve1:create"

	for _, tc := range []struct {
		name string
		wait func() error
	}{
		{"quiet", func() error { return pollTaskQuiet(ctx, client, "pve1", upid, "creating VM", "created VM") }},
		{"interactive", func() error { return watchTask(ctx, client, "pve1", upid, "creating VM", "created VM") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.wait()
			if !errors.Is(err, context.Canceled) {
				t.Errorf("wait error = %v, want context.Canceled", err)
			}
			if err == nil || !strings.Contains(err.Error(), upid) {
				t.Errorf("wait error = %v, want UPID %s", err, upid)
			}
		})
	}
}

func TestTaskWaitStopsAfterPersistentPollFailures(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message":"node unavailable"}`))
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := pollTaskQuiet(ctx, client, "pve1", "UPID:pve1:create", "creating VM", "created VM")
	if err == nil || !strings.Contains(err.Error(), "node unavailable") || !strings.Contains(err.Error(), "UPID:pve1:create") {
		t.Errorf("wait error = %v, want last polling error and UPID", err)
	}
	if requests != 3 {
		t.Errorf("status requests = %d, want 3 before failing", requests)
	}
}

func TestTaskWaitRecoversAfterTransientPollError(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"message":"brief outage"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "stopped", "exitstatus": "OK"}})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pollTaskQuiet(ctx, client, "pve1", "UPID:pve1:create", "creating VM", "created VM"); err != nil {
		t.Errorf("wait error = %v, want recovery", err)
	}
	if requests != 2 {
		t.Errorf("status requests = %d, want 2", requests)
	}
}

func TestTaskWaitTimeoutRetainsUPID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	client := api.NewClient("http://127.0.0.1:1", "user@pve!test", "secret", true)
	err := pollTaskQuiet(ctx, client, "pve1", "UPID:pve1:create", "creating VM", "created VM")
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "UPID:pve1:create") {
		t.Errorf("wait error = %v, want deadline and UPID", err)
	}
}

func TestRunProgressActionHonorsWaitTimeout(t *testing.T) {
	oldTimeout := taskWaitTimeout
	taskWaitTimeout = 20 * time.Millisecond
	defer func() { taskWaitTimeout = oldTimeout }()
	client := api.NewClient("http://127.0.0.1:1", "user@pve!test", "secret", true)
	err := runProgressAction(client, "pve1", "UPID:pve1:create", "creating VM", "created VM")
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "UPID:pve1:create") {
		t.Errorf("runProgressAction() error = %v, want deadline and UPID", err)
	}
}

func TestNegativeWaitTimeoutIsRejected(t *testing.T) {
	oldTimeout := taskWaitTimeout
	taskWaitTimeout = -time.Second
	defer func() { taskWaitTimeout = oldTimeout }()
	if err := validateOutputFormat(rootCmd, nil); err == nil || !strings.Contains(err.Error(), "--wait-timeout") {
		t.Errorf("validateOutputFormat() error = %v, want --wait-timeout validation", err)
	}
}
