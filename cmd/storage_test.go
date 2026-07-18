package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestUsePercent(t *testing.T) {
	tests := []struct {
		name string
		s    api.StorageResource
		want int
	}{
		{"quarter full", api.StorageResource{Disk: 128849018880, MaxDisk: 536870912000}, 24},
		{"no capacity reported", api.StorageResource{Disk: 100, MaxDisk: 0}, 0},
		{"full", api.StorageResource{Disk: 100, MaxDisk: 100}, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usePercent(tt.s); got != tt.want {
				t.Errorf("usePercent(%+v) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}

func TestRenderStorageWarningsNoneBelowThreshold(t *testing.T) {
	storages := []api.StorageResource{
		{Name: "local", Disk: 50, MaxDisk: 100},
	}
	if got := renderStorageWarnings(storages); got != "" {
		t.Errorf("renderStorageWarnings() = %q, want empty for a storage below threshold", got)
	}
}

func TestRenderStorageWarningsGlyphsAndOrdering(t *testing.T) {
	storages := []api.StorageResource{
		{Name: "local-lvm", Disk: 85, MaxDisk: 100}, // 85% — warn
		{Name: "moredata", Disk: 95, MaxDisk: 100},  // 95% — critical
		{Name: "backups", Disk: 12, MaxDisk: 100},   // below threshold, excluded
	}

	got := renderStorageWarnings(storages)

	if strings.Contains(got, "backups") {
		t.Errorf("renderStorageWarnings() = %q, want backups (12%%) excluded", got)
	}
	if !strings.Contains(got, "🚨 moredata 95%") {
		t.Errorf("renderStorageWarnings() = %q, want a 🚨 line for moredata at 95%%", got)
	}
	if !strings.Contains(got, "⚠ local-lvm 85%") {
		t.Errorf("renderStorageWarnings() = %q, want a ⚠ line for local-lvm at 85%%", got)
	}
	if strings.Index(got, "moredata") > strings.Index(got, "local-lvm") {
		t.Errorf("renderStorageWarnings() = %q, want the worst (moredata) listed first", got)
	}
}

func TestRenderStorageReportIncludesUsePercent(t *testing.T) {
	storages := []api.StorageResource{
		{Name: "local-lvm", Node: "pve1", Disk: 128849018880, MaxDisk: 536870912000, Health: "available"},
	}

	got := renderStorageReport(storages)

	if !strings.Contains(got, "24%") {
		t.Errorf("renderStorageReport() = %q, want it to contain the USE%% column (24%%)", got)
	}
}

// TestRenderStorageReportKeepsNonSharedSameNameSeparate guards against a
// real bug: non-shared per-node storage (the default "local"/
// "local-lvm" every node has its own copy of) was being collapsed like
// genuinely shared storage, silently dropping every node past the first
// from the report. Each node's own local-lvm must appear as its own row.
func TestRenderStorageReportKeepsNonSharedSameNameSeparate(t *testing.T) {
	storages := []api.StorageResource{
		{Name: "local-lvm", Node: "pve1", Disk: 128849018880, MaxDisk: 536870912000, Health: "available", Shared: false},
		{Name: "local-lvm", Node: "pve2", Disk: 999999999999, MaxDisk: 999999999999, Health: "available", Shared: false},
	}

	got := renderStorageReport(storages)

	if !strings.Contains(got, "local-lvm@pve1") || !strings.Contains(got, "local-lvm@pve2") {
		t.Errorf("renderStorageReport() = %q, want both nodes' local-lvm listed as separate, disambiguated rows", got)
	}
	if !strings.Contains(got, "24%") || !strings.Contains(got, "100%") {
		t.Errorf("renderStorageReport() = %q, want both nodes' usage percentages present", got)
	}
}

// TestRenderStorageReportCollapsesSharedStorage covers the opposite,
// still-correct case: Proxmox-marked-shared storage really is the same
// pool on every node, so collapsing it to one row is accurate.
func TestRenderStorageReportCollapsesSharedStorage(t *testing.T) {
	storages := []api.StorageResource{
		{Name: "nfs-bulk", Node: "pve1", Disk: 128849018880, MaxDisk: 536870912000, Health: "available", Shared: true},
		{Name: "nfs-bulk", Node: "pve2", Disk: 128849018880, MaxDisk: 536870912000, Health: "available", Shared: true},
	}

	got := renderStorageReport(storages)

	if strings.Count(got, "nfs-bulk") != 1 {
		t.Errorf("renderStorageReport() = %q, want shared nfs-bulk collapsed to a single row", got)
	}
}

func TestStorageCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"storage"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("storage") error = %v`, err)
	}
	if found.Use != "storage" {
		t.Errorf(`Find("storage").Use = %q, want "storage"`, found.Use)
	}
}

func TestRunStorage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"type": "storage", "storage": "local", "node": "pve1", "status": "available", "disk": 128849018880, "maxdisk": 536870912000, "plugintype": "dir"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if err := runStorage(client); err != nil {
		t.Fatalf("runStorage() error = %v", err)
	}
}
