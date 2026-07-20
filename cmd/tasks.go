package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/spf13/cobra"
)

// tasksWatchInterval is how often `pvectl tasks list --watch` refreshes. Same
// cadence as statusWatchInterval — fast enough to feel live without
// hammering the Proxmox API.
const tasksWatchInterval = 2 * time.Second

var tasksWatch bool

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Manage cluster tasks",
}

var tasksListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List recent cluster tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		if tasksWatch {
			return watchTasks(client)
		}
		return runTasks(client)
	},
}

func init() {
	tasksListCmd.Flags().BoolVarP(&tasksWatch, "watch", "w", false, "refresh the task list every 2 seconds until interrupted (Ctrl-C)")
	rootCmd.AddCommand(tasksCmd)
	tasksCmd.AddCommand(tasksListCmd)
}

func runTasks(client *api.Client) error {
	tasks, err := client.ClusterTasks(context.Background())
	if err != nil {
		return fmt.Errorf("fetching cluster tasks: %w", err)
	}
	fmt.Print(renderTasksTable(tasks, verbose))
	return nil
}

// watchTasks re-runs runTasks every tasksWatchInterval, redrawing in place
// between refreshes — same home-cursor/redraw/trim technique as
// watchStatus, so a growing/shrinking task list doesn't leave stale lines
// from a previous, longer frame. A single fetch error doesn't abort the
// loop; it's printed and the next tick retries, riding out a transient
// blip the same way watchStatus does.
func watchTasks(client *api.Client) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h")

	ticker := time.NewTicker(tasksWatchInterval)
	defer ticker.Stop()

	for {
		fmt.Print("\033[H")
		if err := runTasks(client); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		fmt.Printf("\nRefreshing every %s — press Ctrl-C to stop.\n", tasksWatchInterval)
		fmt.Print("\033[J")

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// taskTypeLabels maps Proxmox's task-registry type strings — stable,
// structured API fields, not free-form log text — to a human
// description. qm*/vz* is Proxmox's own long-standing prefix for
// qemu/container tasks respectively (vz predates LXC, from OpenVZ).
// An unrecognized type falls back to its raw string rather than
// guessing, same discipline as TaskLog's "display as-is" rule.
var taskTypeLabels = map[string]string{
	"qmigrate":   "migrate VM",
	"qmstart":    "start VM",
	"qmstop":     "stop VM",
	"qmshutdown": "shutdown VM",
	"qmreboot":   "reboot VM",
	"qmsnapshot": "snapshot VM",
	"qmdestroy":  "destroy VM",
	"qmclone":    "clone VM",
	"qmrestore":  "restore VM",
	"qmcreate":   "create VM",
	"vzmigrate":  "migrate CT",
	"vzstart":    "start CT",
	"vzstop":     "stop CT",
	"vzshutdown": "shutdown CT",
	"vzreboot":   "reboot CT",
	"vzsnapshot": "snapshot CT",
	"vzdestroy":  "destroy CT",
	"vzclone":    "clone CT",
	"vzrestore":  "restore CT",
	"vzcreate":   "create CT",
	"vzdump":     "backup",
}

// taskDescription renders t's type+id as a human-readable description,
// e.g. "migrate VM 101".
func taskDescription(t api.ClusterTask) string {
	label, ok := taskTypeLabels[t.Type]
	if !ok {
		label = t.Type
	}
	if t.ID == "" {
		return label
	}
	return fmt.Sprintf("%s %s", label, t.ID)
}

// taskStatusLabel renders t's status column: "running" while in
// progress, "OK" on a clean finish, "warning: <reason>" for Proxmox's
// non-fatal "WARNINGS: N" exit status, or "failed: <reason>" for anything
// else — same Done/Failed/TaskCompletedWithWarnings semantics as
// TaskStatus, just read directly from the cluster task list instead of a
// follow-up status poll.
func taskStatusLabel(t api.ClusterTask) string {
	if t.Running() {
		return "running"
	}
	if t.Status == "OK" {
		return "OK"
	}
	if api.TaskCompletedWithWarnings(t.Status) {
		return "warning: " + t.Status
	}
	return "failed: " + t.Status
}

// formatTaskTime renders a unix timestamp as a local date+time, or "-"
// for the zero value (an EndTime that hasn't happened yet).
func formatTaskTime(unix int64) string {
	if unix == 0 {
		return "-"
	}
	return time.Unix(unix, 0).Local().Format("2006-01-02 15:04:05")
}

// renderTasksTable formats tasks as a START/END/NODE/USER/DESCRIPTION/
// STATUS table, oldest first (newest at the bottom, near the prompt —
// easier to read than scrolling up for the latest), from already-fetched
// data — pure and unit-testable, same pattern as renderNodes/
// renderStorageReport. UPIDs are an extra trailing column shown only
// under verbose, matching the UPID-hidden-by-default convention used by
// the action progress views.
func renderTasksTable(tasks []api.ClusterTask, verbose bool) string {
	sorted := append([]api.ClusterTask(nil), tasks...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartTime < sorted[j].StartTime
	})

	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

	header := "START\tEND\tNODE\tUSER\tDESCRIPTION\tSTATUS"
	if verbose {
		header += "\tUPID"
	}
	_, _ = fmt.Fprintln(tw, header)

	for _, t := range sorted {
		row := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s",
			formatTaskTime(t.StartTime), formatTaskTime(t.EndTime),
			t.Node, t.User, taskDescription(t), taskStatusLabel(t))
		if verbose {
			row += "\t" + t.UPID
		}
		_, _ = fmt.Fprintln(tw, row)
	}

	_ = tw.Flush()
	return buf.String()
}
