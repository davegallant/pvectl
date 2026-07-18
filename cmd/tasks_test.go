package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestTaskDescriptionKnownType(t *testing.T) {
	got := taskDescription(api.ClusterTask{Type: "qmigrate", ID: "101"})
	if got != "migrate VM 101" {
		t.Errorf("taskDescription() = %q, want %q", got, "migrate VM 101")
	}
}

func TestTaskDescriptionUnknownTypeFallsBackToRaw(t *testing.T) {
	got := taskDescription(api.ClusterTask{Type: "srvstart", ID: "pveproxy"})
	if got != "srvstart pveproxy" {
		t.Errorf("taskDescription() = %q, want %q", got, "srvstart pveproxy")
	}
}

func TestTaskStatusLabel(t *testing.T) {
	tests := []struct {
		name string
		t    api.ClusterTask
		want string
	}{
		{"running", api.ClusterTask{EndTime: 0}, "running"},
		{"ok", api.ClusterTask{EndTime: 100, Status: "OK"}, "OK"},
		{"warning", api.ClusterTask{EndTime: 100, Status: "WARNINGS: 1"}, "warning: WARNINGS: 1"},
		{"failed", api.ClusterTask{EndTime: 100, Status: "unable to open file"}, "failed: unable to open file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := taskStatusLabel(tt.t); got != tt.want {
				t.Errorf("taskStatusLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderTasksTableSortsOldestFirstAndHidesUPIDByDefault(t *testing.T) {
	tasks := []api.ClusterTask{
		{UPID: "UPID:pve1:1", Node: "pve1", User: "root@pam", Type: "vzstart", ID: "201", StartTime: 1000, EndTime: 1010, Status: "OK"},
		{UPID: "UPID:pve1:2", Node: "pve1", User: "root@pam", Type: "qmigrate", ID: "101", StartTime: 2000},
	}

	got := renderTasksTable(tasks, false)

	if strings.Contains(got, "UPID:pve1") {
		t.Errorf("renderTasksTable(verbose=false) = %q, want no UPID column", got)
	}
	if strings.Index(got, "start CT 201") > strings.Index(got, "migrate VM 101") {
		t.Errorf("renderTasksTable() = %q, want the newer task (migrate VM 101) listed last", got)
	}
	if !strings.Contains(got, "running") {
		t.Errorf("renderTasksTable() = %q, want the still-running task marked \"running\"", got)
	}
}

func TestRenderTasksTableVerboseShowsUPID(t *testing.T) {
	tasks := []api.ClusterTask{
		{UPID: "UPID:pve1:1", Node: "pve1", User: "root@pam", Type: "vzstart", ID: "201", StartTime: 1000, EndTime: 1010, Status: "OK"},
	}

	got := renderTasksTable(tasks, true)

	if !strings.Contains(got, "UPID:pve1:1") {
		t.Errorf("renderTasksTable(verbose=true) = %q, want the UPID column present", got)
	}
}

func TestTasksCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"tasks"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("tasks") error = %v`, err)
	}
	if found.Use != "tasks" {
		t.Errorf(`Find("tasks").Use = %q, want "tasks"`, found.Use)
	}
}

func TestRunTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"upid": "UPID:pve1:1", "node": "pve1", "user": "root@pam", "type": "vzstart", "id": "201", "starttime": 1000, "endtime": 1010, "status": "OK"},
			},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "user@pve!test", "secret", true)
	if err := runTasks(client); err != nil {
		t.Fatalf("runTasks() error = %v", err)
	}
}
