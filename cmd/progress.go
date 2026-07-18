package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/davegallant/pvectl/internal/api"
	"golang.org/x/term"
)

// spinnerFrames cycles once per frameInterval while a task is in flight.
var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

const (
	// frameInterval is how often the spinner animates.
	frameInterval = 100 * time.Millisecond
	// taskPollInterval is how often Proxmox's task status is polled — much
	// slower than the spinner redraw so a live view doesn't hammer the API
	// for one in-flight task (cf. statusWatchInterval for the whole-cluster
	// `status --watch` case, which polls even less often since it's a
	// heavier fetch).
	taskPollInterval = time.Second
)

// isInteractive reports whether stdout is a terminal. The live spinner view
// only makes sense there; piped/scripted output gets a quiet polled result
// instead, so composing pvectl with other tools isn't regressed by terminal
// escape codes. Checked once, in runProgressAction, rather than at each call
// site.
func isInteractive() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// formatProgressLine renders one frame of the in-flight spinner line.
func formatProgressLine(frame rune, elapsed time.Duration, label string) string {
	return fmt.Sprintf("%c %s… %s", frame, label, elapsed.Round(time.Second))
}

// formatDoneLine renders the final line once a task finishes successfully.
// The UPID is only appended under --verbose — most users don't need it,
// but it's useful for cross-referencing Proxmox's own Task History/logs.
func formatDoneLine(verbDone string, elapsed time.Duration, verbose bool, upid string) string {
	line := fmt.Sprintf("✓ %s (%s)", verbDone, elapsed.Round(100*time.Millisecond))
	if verbose {
		line += fmt.Sprintf(" [%s]", upid)
	}
	return line
}

// formatFailLine renders the final line once a task finishes with a
// non-OK exit status.
func formatFailLine(verbDone string, elapsed time.Duration, exitStatus string, verbose bool, upid string) string {
	line := fmt.Sprintf("✗ %s failed after %s: %s", verbDone, elapsed.Round(100*time.Millisecond), exitStatus)
	if verbose {
		line += fmt.Sprintf(" [%s]", upid)
	}
	return line
}

// printTaskLogLines fetches and prints upid's real Proxmox task log —
// Proxmox's own free-form, version-dependent text, displayed as-is per
// TaskLog's own documented policy, not parsed. A log-fetch failure is
// reported to stderr and swallowed rather than propagated: this is
// best-effort diagnostic output layered on top of an outcome that's
// already been decided, not something that should itself fail the
// command.
func printTaskLogLines(client *api.Client, node, upid string) {
	lines, err := client.TaskLog(context.Background(), node, upid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: fetching task log: %v\n", err)
		return
	}
	fmt.Println("\ntask log:")
	for _, l := range lines {
		fmt.Println(l.T)
	}
}

// renderTaskOutcome prints the final done/fail line for a finished task and
// returns the error (if any) the caller should propagate: nil on success,
// a non-nil error on failure so pvectl exits non-zero. Shared by the
// interactive (spinner) and non-interactive (quietly-polled) completion
// paths so a failed-but-successfully-triggered task makes *both* exit
// non-zero — a scripted `pvectl ct start <broken>` used to print
// "started …" and exit 0 because the non-tty path returned nil the instant
// the trigger POST came back, without ever polling the task's real outcome.
//
// On failure, the real task log is also printed (unconditionally, not
// gated on --verbose) — Proxmox's ExitStatus is sometimes just
// "WARNINGS: 1" with no further detail in the status reply itself, which
// told a user nothing about what actually went wrong (found via a real
// `ct create` that failed this way with an otherwise-successfully-created
// container). The log is what has the actual warning/error text.
//
// A "WARNINGS: N" exit status specifically isn't Failed() (see
// TaskCompletedWithWarnings) — Proxmox itself treats that as a completed
// task with caveats, not a failure (confirmed via a real `ct create` whose
// only "warning" was a systemd/nesting hint on an otherwise fully-created
// container). It still prints the ✓ done line and returns nil, but the
// log is shown anyway so the caveat isn't silently swallowed.
func renderTaskOutcome(client *api.Client, node, verbDone string, elapsed time.Duration, status api.TaskStatus, verbose bool, upid string) error {
	if status.Failed() {
		fmt.Println(formatFailLine(verbDone, elapsed, status.ExitStatus, verbose, upid))
		printTaskLogLines(client, node, upid)
		return fmt.Errorf("task %s failed: %s", upid, status.ExitStatus)
	}
	fmt.Println(formatDoneLine(verbDone, elapsed, verbose, upid))
	if status.ExitStatus != "OK" {
		printTaskLogLines(client, node, upid)
	}
	return nil
}

// runProgressAction shows a live spinner while the task named by upid runs
// on node, then a final ✓/✗ line, when stdout is a terminal. Otherwise it
// quietly polls the task to completion (no spinner, no escape codes — just
// polling so a script still gets the real outcome) and prints the same
// final line. In both modes a failed task returns a non-nil error, so
// `pvectl` exits non-zero when a triggered task fails, instead of only when
// the trigger POST itself fails.
func runProgressAction(client *api.Client, node, upid, label, verbDone string) error {
	if !isInteractive() {
		return pollTaskQuiet(client, node, upid, label, verbDone)
	}
	return watchTask(context.Background(), client, node, upid, label, verbDone)
}

// watchTask polls upid's status on node until it finishes, redrawing a
// single spinner line in place — the same hide-cursor/redraw/restore
// technique `pvectl status --watch`'s watchStatus established, but for one
// line (`\r` + `\033[K`) rather than a whole multi-line frame (`\033[H`/
// `\033[J`). Ctrl-C stops watching only; the task keeps running on Proxmox
// untouched, and the UPID is printed unconditionally (regardless of
// --verbose) so it can be checked on later.
func watchTask(ctx context.Context, client *api.Client, node, upid, label, verbDone string) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h")

	start := time.Now()
	frameTicker := time.NewTicker(frameInterval)
	defer frameTicker.Stop()
	pollTicker := time.NewTicker(taskPollInterval)
	defer pollTicker.Stop()

	frame := 0
	redraw := func() {
		fmt.Printf("\r\033[K%s", formatProgressLine(spinnerFrames[frame%len(spinnerFrames)], time.Since(start), label))
	}
	redraw()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\r\033[K%s — still running on Proxmox (upid %s)\n", label, upid)
			return nil
		case <-pollTicker.C:
			status, err := client.TaskStatus(ctx, node, upid)
			if err != nil {
				// A transient poll failure doesn't end the watch — the
				// task itself is unaffected server-side. Keep spinning
				// and retry on the next tick, same "ride out a blip"
				// policy as status --watch.
				continue
			}
			if !status.Done() {
				continue
			}
			fmt.Print("\r\033[K")
			return renderTaskOutcome(client, node, verbDone, time.Since(start), status, verbose, upid)
		case <-frameTicker.C:
			frame++
			redraw()
		}
	}
}

// pollTaskQuiet is the non-interactive (piped/scripted) completion path:
// polls upid on node at taskPollInterval until it finishes, without any
// spinner or terminal escape codes, then prints the same final ✓/✗ line as
// the interactive path and returns a non-nil error on failure. This is the
// fix for the scripted case that used to print the done-verb and exit 0 the
// instant the trigger POST returned, never learning whether the task
// actually succeeded. A transient poll error is ridden out (continue), same
// as the interactive path and `status --watch`. Ctrl-C stops polling only
// — the task keeps running on Proxmox, and the UPID is printed so it can be
// checked on later.
//
// Blocking to completion in this mode also incidentally fixes a
// migrate-specific ordering bug: runMigrate calls runProgressAction *then*
// printTaskLogIfVerbose, so before this change a scripted
// `ct migrate --verbose` dumped an empty/partial task log while the task was
// still in flight; now the log is fetched only after the task is done.
func pollTaskQuiet(client *api.Client, node, upid, label, verbDone string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ticker := time.NewTicker(taskPollInterval)
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("%s — still running on Proxmox (upid %s)\n", label, upid)
			return nil
		case <-ticker.C:
			status, err := client.TaskStatus(ctx, node, upid)
			if err != nil {
				continue
			}
			if !status.Done() {
				continue
			}
			return renderTaskOutcome(client, node, verbDone, time.Since(start), status, verbose, upid)
		}
	}
}
