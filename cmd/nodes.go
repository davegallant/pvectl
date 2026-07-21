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
	Short: "Manage Proxmox cluster nodes",
}

var nodesListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List Proxmox cluster nodes",
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
	nodesCmd.AddCommand(nodesListCmd)
}

// runNodes fetches the two independent endpoints `pvectl nodes list` needs
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

	if jsonOutput {
		return printJSON(nodesJSON(status, resources.Nodes))
	}
	fmt.Print(renderNodes(status, resources.Nodes))
	return nil
}

// nodeJSON is one node's `nodes list --json` entry, joining the IP from
// ClusterStatus with the CPU/mem usage from ClusterResources — the same
// two sources renderNodesTable's columns come from.
type nodeJSON struct {
	Name   string  `json:"name"`
	IP     string  `json:"ip,omitempty"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"` // fraction 0-1
	Mem    int64   `json:"mem"`
	MaxMem int64   `json:"maxMem"`
}

// nodesJSON builds nodeJSON entries from already-fetched data, sorted by
// name — mirrors renderNodes's sort so JSON and table output agree on
// order.
func nodesJSON(status api.ClusterStatus, nodeResources []api.NodeResource) []nodeJSON {
	nodes := append([]api.NodeResource(nil), nodeResources...)
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})

	out := make([]nodeJSON, 0, len(nodes))
	for _, n := range nodes {
		ip := ""
		if ns, ok := status.Nodes[n.Name]; ok {
			ip = ns.IP
		}
		out = append(out, nodeJSON{
			Name:   n.Name,
			IP:     ip,
			Status: n.Status,
			CPU:    n.CPU,
			Mem:    n.Mem,
			MaxMem: n.MaxMem,
		})
	}
	return out
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
