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

	runErr = pollTaskQuiet(client, node, upid, label, verbDone)

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
