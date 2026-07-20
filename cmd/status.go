package cmd

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/davegallant/pvectl/internal/api"
	"github.com/spf13/cobra"
)

// okStyle/warnStyle/errStyle/infoStyle color the per-check status marks in
// `pvectl status`: green ✓ for a passing check, yellow ⚠ for an advisory
// warning, red ✗ for a problem, and a dim • for informational counts that
// aren't a health verdict (guest running/stopped tallies — a stopped guest
// is often intentional, not a fault). lipgloss auto-detects whether stdout
// is a real terminal and strips the escape codes when it isn't, so piped
// output and tests see plain glyphs.
var (
	okStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// Per-check outcome levels. CTs/VMs counts aren't a pass/fail verdict, so
// they render via infoIcon's "•" glyph directly rather than one of these.
const (
	checkOK = iota
	checkWarn
	checkErr
)

func statusIcon(level int) string {
	switch level {
	case checkWarn:
		return warnStyle.Render("⚠")
	case checkErr:
		return errStyle.Render("✗")
	default:
		return okStyle.Render("✓")
	}
}

func infoIcon() string { return infoStyle.Render("•") }

// formatBytes renders n as a human-readable size using binary (1024-based)
// units with single-letter suffixes (K/M/G/T/P/E), one decimal place with
// a trailing ".0" dropped (e.g. "120G", "1.5K", not "120.0G").
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	units := "KMGTPE"
	value := float64(n) / float64(div)
	s := strconv.FormatFloat(value, 'f', 1, 64)
	s = strings.TrimSuffix(s, ".0")
	return fmt.Sprintf("%s%c", s, units[exp])
}

// renderStatus formats a compact cluster health summary: the version/
// quorum header plus a short line of sanity checks (quorum, node online
// state, guest counts, storage health/availability) and an overall
// verdict line. The detailed per-resource tables live on `pvectl nodes list`
// and `pvectl storage list` — `pvectl status` is the at-a-glance view. It
// performs no I/O, so it's directly unit-testable.
func renderStatus(version string, status api.ClusterStatus, resources api.ClusterResources) string {
	var buf strings.Builder

	if status.Standalone {
		fmt.Fprintf(&buf, "Proxmox VE %s — standalone\n\n", version)
	} else {
		quorate := "not quorate"
		if status.Quorate {
			quorate = "quorate"
		}
		fmt.Fprintf(&buf, "Proxmox VE %s — cluster: %s (%s)\n\n", version, status.Name, quorate)
	}

	// Track the worst status seen and how many checks aren't fully OK so
	// the verdict line at the bottom reflects the overall picture.
	worst := checkOK
	issues := 0
	add := func(level int, label, detail string) {
		if level != checkOK {
			issues++
			if level > worst {
				worst = level
			}
		}
		fmt.Fprintf(&buf, "%s %-7s  %s\n", statusIcon(level), label, detail)
	}
	info := func(label, detail string) {
		fmt.Fprintf(&buf, "%s %-7s  %s\n", infoIcon(), label, detail)
	}

	// Quorum — cluster only; a standalone host has no quorum concept.
	if !status.Standalone {
		if status.Quorate {
			add(checkOK, "Quorum", "quorate")
		} else {
			add(checkErr, "Quorum", "not quorate")
		}
	}

	// Nodes online. A node's status string comes from /cluster/resources.
	total := len(resources.Nodes)
	online := 0
	for _, n := range resources.Nodes {
		if n.Status == "online" {
			online++
		}
	}
	level := checkOK
	if online != total {
		level = checkErr
	}
	add(level, "Nodes", fmt.Sprintf("%d/%d online", online, total))

	// Guest counts are informational, not a health verdict — a stopped
	// container/VM is often intentional, not a problem.
	info("CTs", fmt.Sprintf("%d running, %d stopped", resources.Containers.Running, resources.Containers.Stopped))
	info("VMs", fmt.Sprintf("%d running, %d stopped", resources.VMs.Running, resources.VMs.Stopped))

	// Storage: availability first, then capacity pressure (reusing the
	// warn/crit thresholds shared with `pvectl storage list`).
	storage := sortedCollapsedStorage(resources.Storage)
	sTotal := len(storage)
	sAvail := 0
	var bad []string
	worstPct := 0
	worstPctName := ""
	for _, s := range storage {
		if s.Health == "available" {
			sAvail++
		} else {
			bad = append(bad, s.Name)
		}
		if p := usePercent(s); p > worstPct {
			worstPct = p
			worstPctName = s.Name
		}
	}
	switch {
	case sAvail < sTotal:
		add(checkErr, "Storage", fmt.Sprintf("%d/%d available (%s)", sAvail, sTotal, strings.Join(bad, ", ")))
	case worstPct >= storageCritPercent:
		add(checkErr, "Storage", fmt.Sprintf("%d/%d available — %s at %d%%", sAvail, sTotal, worstPctName, worstPct))
	case worstPct >= storageWarnPercent:
		add(checkWarn, "Storage", fmt.Sprintf("%d/%d available — %s at %d%%", sAvail, sTotal, worstPctName, worstPct))
	default:
		add(checkOK, "Storage", fmt.Sprintf("%d/%d available", sAvail, sTotal))
	}

	if worst == checkOK {
		fmt.Fprintf(&buf, "\n%s Cluster is healthy\n", statusIcon(checkOK))
	} else {
		noun := "issues"
		if issues == 1 {
			noun = "issue"
		}
		fmt.Fprintf(&buf, "\n%s %d %s need attention\n", statusIcon(worst), issues, noun)
	}

	return buf.String()
}

// sortedCollapsedStorage sorts a copy of storages by name (ties broken by
// node) and collapses entries sharing a name down to one row each — shared
// by `pvectl status`'s Storage section and the standalone `pvectl storage
// list` command so they can't drift apart on the same underlying data.
func sortedCollapsedStorage(storages []api.StorageResource) []api.StorageResource {
	storage := append([]api.StorageResource(nil), storages...)
	sort.Slice(storage, func(i, j int) bool {
		if storage[i].Name != storage[j].Name {
			return storage[i].Name < storage[j].Name
		}
		return storage[i].Node < storage[j].Node
	})
	return collapseStorageByName(storage)
}

// usePercent returns s's used-space percentage, rounded to the nearest
// whole number. Returns 0 for a storage with no reported capacity rather
// than dividing by zero.
func usePercent(s api.StorageResource) int {
	if s.MaxDisk <= 0 {
		return 0
	}
	return int(math.Round(float64(s.Disk) / float64(s.MaxDisk) * 100))
}

// renderStorageTable writes the NAME/USED/TOTAL/USE/HEALTH table for
// storages to w. Shared by `pvectl status` (as one section among several)
// and `pvectl storage list` (as its whole output). storages should already be
// deduplicated by name (see sortedCollapsedStorage) — mirrors
// renderNodesTable's role for nodes.
func renderStorageTable(w io.Writer, storages []api.StorageResource) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tUSED\tTOTAL\tUSE\tHEALTH")
	for _, s := range storages {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%d%%\t%s\n", s.Name, formatBytes(s.Disk), formatBytes(s.MaxDisk), usePercent(s), storageHealth(s.Health))
	}
	_ = tw.Flush()
}

// storageWarnPercent/storageCritPercent are the usage thresholds at which
// renderStorageWarnings flags a storage as running low on space.
const (
	storageWarnPercent = 85
	storageCritPercent = 95
)

// renderStorageWarnings returns one "⚠"/"🚨 name NN%" line per storage at
// or above storageWarnPercent, worst first, or "" if none qualify — a
// quick, easy-to-spot summary on top of the full table's numbers.
func renderStorageWarnings(storages []api.StorageResource) string {
	type warning struct {
		name    string
		percent int
	}
	var warnings []warning
	for _, s := range storages {
		if p := usePercent(s); p >= storageWarnPercent {
			warnings = append(warnings, warning{s.Name, p})
		}
	}
	if len(warnings) == 0 {
		return ""
	}
	sort.Slice(warnings, func(i, j int) bool { return warnings[i].percent > warnings[j].percent })

	var b strings.Builder
	for _, w := range warnings {
		icon := "⚠"
		if w.percent >= storageCritPercent {
			icon = "🚨"
		}
		fmt.Fprintf(&b, "%s %s %d%%\n", icon, w.name, w.percent)
	}
	return b.String()
}

// renderNodesTable writes the NAME/IP/STATUS/CPU/MEM table for nodes to w.
// Shared by `pvectl status` (as one section among several) and `pvectl
// nodes` (as its whole output). nodes must already be sorted by name.
func renderNodesTable(w io.Writer, status api.ClusterStatus, nodes []api.NodeResource) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tIP\tSTATUS\tCPU\tMEM")
	for _, n := range nodes {
		ip := "-"
		if ns, ok := status.Nodes[n.Name]; ok && ns.IP != "" {
			ip = ns.IP
		}
		cpuPct := int(math.Round(n.CPU * 100))
		memPct := 0
		if n.MaxMem > 0 {
			memPct = int(math.Round(float64(n.Mem) / float64(n.MaxMem) * 100))
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%d%%\t%d%%\n", n.Name, ip, n.Status, cpuPct, memPct)
	}
	_ = tw.Flush()
}

// collapseStorageByName collapses storage entries sharing a name down to
// one row each, but only when they're actually the same storage — i.e.
// Proxmox's own Shared flag is set (an NFS/Ceph pool mounted identically
// on every node). Non-shared entries that merely share a name (every
// node's own separate "local"/"local-lvm") are never collapsed: doing so
// used to silently drop every node past the first from the report, which
// looked like "local"/"local-lvm" vanishing entirely on a multi-node
// cluster. Those are instead disambiguated as "name@node" so each node's
// real capacity still shows up as its own row. storage must already be
// sorted by name (ties broken by node) so "first seen" is deterministic.
func collapseStorageByName(storage []api.StorageResource) []api.StorageResource {
	nonSharedCount := map[string]int{}
	for _, s := range storage {
		if !s.Shared {
			nonSharedCount[s.Name]++
		}
	}

	var out []api.StorageResource
	for _, s := range storage {
		if !s.Shared {
			if nonSharedCount[s.Name] > 1 {
				s.Name = fmt.Sprintf("%s@%s", s.Name, s.Node)
			}
			out = append(out, s)
			continue
		}
		if len(out) > 0 && out[len(out)-1].Name == s.Name && out[len(out)-1].Shared {
			continue
		}
		out = append(out, s)
	}
	return out
}

// storageHealth maps Proxmox's raw storage status to a short display
// value: "available" (the common case) becomes "OK"; anything else
// (including empty) is shown as-is, uppercased, so a real problem isn't
// silently normalized away.
func storageHealth(status string) string {
	if status == "available" {
		return "OK"
	}
	if status == "" {
		return "UNKNOWN"
	}
	return strings.ToUpper(status)
}

// statusWatchInterval is how often `pvectl status --watch` refreshes.
// Not a flag — 2s is fast enough to feel live without hammering the
// Proxmox API on every keystroke-equivalent tick.
const statusWatchInterval = 2 * time.Second

var statusWatch bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a quick Proxmox cluster health summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		if statusWatch {
			return watchStatus(client)
		}
		return runStatus(client)
	},
}

func init() {
	statusCmd.Flags().BoolVarP(&statusWatch, "watch", "w", false, "refresh the status output every 2 seconds until interrupted (Ctrl-C)")
	rootCmd.AddCommand(statusCmd)
}

// watchStatus re-runs runStatus every statusWatchInterval, redrawing in
// place between refreshes (like the Unix `watch` command), until the user
// interrupts with Ctrl-C. A single fetch error doesn't abort the loop —
// it's printed and the next tick retries, since a watch is expected to
// ride out a transient network blip rather than exit.
func watchStatus(client *api.Client) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Hide the cursor for the duration of the watch — otherwise it sits
	// wherever the last redraw left it and visibly jumps/blinks there on
	// every tick. Always restored on the way out, however we exit.
	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h")

	ticker := time.NewTicker(statusWatchInterval)
	defer ticker.Stop()

	for {
		// Move the cursor home and redraw over the previous frame instead
		// of clearing first (\033[2J) — clearing then redrawing leaves a
		// blank frame on screen between the two, which reads as a flicker
		// every tick. \033[J after the new content trims any leftover
		// lines from a previous, longer frame.
		fmt.Print("\033[H")
		if err := runStatus(client); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		fmt.Printf("\nRefreshing every %s — press Ctrl-C to stop.\n", statusWatchInterval)
		fmt.Print("\033[J")

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// runStatus fetches the three independent endpoints `pvectl status` needs
// (Version, ClusterStatus, ClusterResources) concurrently rather than
// sequentially, cutting the one-shot latency from ~3 round trips to ~1.
// The underlying *http.Client/Transport are documented concurrency-safe,
// so a shared *Client across goroutines is fine; each goroutine writes a
// distinct result/err variable and Wait establishes happens-before. The
// errors are checked in the original Version → ClusterStatus →
// ClusterResources order so a failure still reads naturally (the first
// endpoint that failed is reported first, same as when this was sequential),
// even though all three calls were issued in flight together. See also
// runNodes, which fans out the same two cluster reads the same way.
func runStatus(client *api.Client) error {
	ctx := context.Background()

	var (
		version      string
		status       api.ClusterStatus
		resources    api.ClusterResources
		versionErr   error
		statusErr    error
		resourcesErr error
	)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); version, versionErr = client.Version(ctx) }()
	go func() { defer wg.Done(); status, statusErr = client.ClusterStatus(ctx) }()
	go func() { defer wg.Done(); resources, resourcesErr = client.ClusterResources(ctx) }()
	wg.Wait()

	if versionErr != nil {
		return fmt.Errorf("fetching version: %w", versionErr)
	}
	if statusErr != nil {
		return fmt.Errorf("fetching cluster status: %w", statusErr)
	}
	if resourcesErr != nil {
		return fmt.Errorf("fetching cluster resources: %w", resourcesErr)
	}

	fmt.Print(renderStatus(version, status, resources))
	return nil
}
