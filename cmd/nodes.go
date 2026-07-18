package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/spf13/cobra"
)

var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "List Proxmox cluster nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		return runNodes(client)
	},
}

func init() {
	rootCmd.AddCommand(nodesCmd)
}

// runNodes fetches the two independent endpoints `pvectl nodes` needs
// (ClusterStatus, ClusterResources) concurrently rather than
// sequentially, halving the one-shot latency (~2 round trips → ~1). Same
// shared-Client/concurrency-safety reasoning as runStatus in status.go;
// errors are checked in the original ClusterStatus → ClusterResources
// order so a failure still reports the first endpoint that failed.
func runNodes(client *api.Client) error {
	ctx := context.Background()

	var (
		status    api.ClusterStatus
		resources api.ClusterResources
		statusErr error
		resErr    error
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); status, statusErr = client.ClusterStatus(ctx) }()
	go func() { defer wg.Done(); resources, resErr = client.ClusterResources(ctx) }()
	wg.Wait()

	if statusErr != nil {
		return fmt.Errorf("fetching cluster status: %w", statusErr)
	}
	if resErr != nil {
		return fmt.Errorf("fetching cluster resources: %w", resErr)
	}

	fmt.Print(renderNodes(status, resources.Nodes))
	return nil
}

// renderNodes formats the cluster's node table (NAME/IP/STATUS/CPU/MEM)
// from already-fetched data. It performs no I/O, so it's directly
// unit-testable.
func renderNodes(status api.ClusterStatus, nodeResources []api.NodeResource) string {
	nodes := append([]api.NodeResource(nil), nodeResources...)
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})

	var buf strings.Builder
	renderNodesTable(&buf, status, nodes)
	return buf.String()
}
