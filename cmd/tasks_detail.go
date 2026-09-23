package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/spf13/cobra"
)

var tasksStatusCmd = &cobra.Command{
	Use:         "status <upid>",
	Short:       "Show the current status of a Proxmox task",
	Args:        requireArgs("upid"),
	Annotations: mutationAnnotation(mutationSafe),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		return runTaskStatusContext(cmd.Context(), client, args[0])
	},
}

var tasksLogsCmd = &cobra.Command{
	Use:         "logs <upid>",
	Short:       "Print a Proxmox task's complete log",
	Args:        requireArgs("upid"),
	Annotations: mutationAnnotation(mutationSafe),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		return runTaskLogsContext(cmd.Context(), client, args[0])
	},
}

var tasksWaitCmd = &cobra.Command{
	Use:         "wait <upid>",
	Short:       "Wait for a Proxmox task and report its outcome",
	Args:        requireArgs("upid"),
	Annotations: mutationAnnotation(mutationSafe),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		return runTaskWait(client, args[0])
	},
}

func init() {
	tasksCmd.AddCommand(tasksStatusCmd, tasksLogsCmd, tasksWaitCmd)
}

// taskUPIDNode gets the node embedded in a UPID. Validate before using it
// as a URL path component; the remaining UPID is sent to Proxmox unchanged.
func taskUPIDNode(upid string) (string, error) {
	parts := strings.SplitN(upid, ":", 3)
	if len(parts) != 3 || parts[0] != "UPID" || parts[1] == "" || parts[1] == "." || parts[1] == ".." || parts[2] == "" || strings.ContainsAny(upid, "/?#%") {
		return "", fmt.Errorf("invalid task UPID %q", upid)
	}
	for _, r := range parts[1] {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			continue
		}
		return "", fmt.Errorf("invalid task UPID %q", upid)
	}
	return parts[1], nil
}

func runTaskStatus(client *api.Client, upid string) error {
	return runTaskStatusContext(commandContext(), client, upid)
}

func runTaskStatusContext(ctx context.Context, client *api.Client, upid string) error {
	node, err := taskUPIDNode(upid)
	if err != nil {
		return err
	}
	status, err := client.TaskStatus(ctx, node, upid)
	if err != nil {
		return fmt.Errorf("fetching task %s status: %w", upid, err)
	}
	if jsonOutput {
		return printJSON(status)
	}
	label := "running"
	if status.Done() {
		switch {
		case status.Failed():
			label = "failed: " + status.ExitStatus
		case api.TaskCompletedWithWarnings(status.ExitStatus):
			label = "warning: " + status.ExitStatus
		default:
			label = status.ExitStatus
		}
	}
	fmt.Printf("%s: %s\n", upid, label)
	return nil
}

func runTaskLogs(client *api.Client, upid string) error {
	return runTaskLogsContext(commandContext(), client, upid)
}

func runTaskLogsContext(ctx context.Context, client *api.Client, upid string) error {
	node, err := taskUPIDNode(upid)
	if err != nil {
		return err
	}
	lines, err := client.TaskLog(ctx, node, upid)
	if err != nil {
		return fmt.Errorf("fetching task %s log: %w", upid, err)
	}
	if jsonOutput {
		if lines == nil {
			lines = []api.TaskLogLine{}
		}
		return printJSON(lines)
	}
	for _, line := range lines {
		fmt.Println(line.T)
	}
	return nil
}

func runTaskWait(client *api.Client, upid string) error {
	node, err := taskUPIDNode(upid)
	if err != nil {
		return err
	}
	return runProgressAction(client, node, upid, "waiting for task "+upid, "task "+upid+" completed")
}
