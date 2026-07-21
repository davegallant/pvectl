package cmd

import (
	"context"
	"fmt"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/spf13/cobra"
)

var (
	ctCloneNewID    int
	ctCloneHostname string
	ctCloneStorage  string
	ctCloneFull     bool
	ctCloneTarget   string
	ctClonePool     string
	ctCloneDesc     string
	ctCloneSnapname string
)

var ctCloneCmd = &cobra.Command{
	Use:               "clone <name-or-vmid>",
	Short:             "Clone a container",
	Annotations:       mutationAnnotation(mutationMutating),
	Args:              requireArgs("name-or-vmid"),
	ValidArgsFunction: completeContainerNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		return runCtClone(client, args)
	},
}

// runCtClone resolves the source container from args[0], auto-assigns a
// target vmid via NextID when --newid is unset (0), the same fallback
// ct create's --vmid uses, and clones. Proxmox itself decides full vs.
// linked cloning when --full is omitted (see CloneContainerParams), so
// there's no interactive prompt here — every flag has a sensible default.
func runCtClone(client *api.Client, args []string) error {
	c, err := findContainer(client, args[0])
	if err != nil {
		return err
	}

	newid := ctCloneNewID
	if newid == 0 {
		newid, err = client.NextID(context.Background())
		if err != nil {
			return fmt.Errorf("assigning next free vmid: %w", err)
		}
	}

	params := api.CloneContainerParams{
		TargetVMID:  newid,
		Hostname:    ctCloneHostname,
		Storage:     ctCloneStorage,
		Full:        ctCloneFull,
		Target:      ctCloneTarget,
		Pool:        ctClonePool,
		Description: ctCloneDesc,
		SnapName:    ctCloneSnapname,
	}

	upid, err := client.CloneContainer(context.Background(), c.Node, c.VMID, params)
	if err != nil {
		return fmt.Errorf("cloning %s (%d) to %d: %w", c.Name, c.VMID, newid, err)
	}
	return runProgressAction(client, c.Node, upid,
		fmt.Sprintf("cloning %s (%d) to %d", c.Name, c.VMID, newid),
		fmt.Sprintf("cloned %s (%d) to %d", c.Name, c.VMID, newid))
}

func init() {
	ctCloneCmd.Flags().IntVar(&ctCloneNewID, "newid", 0, "vmid for the clone (0 = auto-assign the next free ID)")
	ctCloneCmd.Flags().StringVar(&ctCloneHostname, "hostname", "", "hostname for the clone (defaults to the source's)")
	ctCloneCmd.Flags().StringVar(&ctCloneStorage, "storage", "", "target storage for a full clone (defaults to the source's storage)")
	ctCloneCmd.Flags().BoolVar(&ctCloneFull, "full", false, "force a full (independent) clone instead of a linked clone")
	ctCloneCmd.Flags().StringVar(&ctCloneTarget, "target", "", "node to create the clone on (defaults to the source's node)")
	ctCloneCmd.Flags().StringVar(&ctClonePool, "pool", "", "resource pool for the clone")
	ctCloneCmd.Flags().StringVar(&ctCloneDesc, "description", "", "description for the clone")
	ctCloneCmd.Flags().StringVar(&ctCloneSnapname, "snapname", "", "clone from this snapshot instead of the container's current state")
	ctCmd.AddCommand(ctCloneCmd)
}
