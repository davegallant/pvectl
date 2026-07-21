package cmd

import (
	"context"
	"fmt"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/spf13/cobra"
)

var (
	qmCloneNewID    int
	qmCloneName     string
	qmCloneStorage  string
	qmCloneFull     bool
	qmCloneTarget   string
	qmClonePool     string
	qmCloneDesc     string
	qmCloneSnapname string
)

var qmCloneCmd = &cobra.Command{
	Use:               "clone <name-or-vmid>",
	Short:             "Clone a VM",
	Annotations:       mutationAnnotation(mutationMutating),
	Args:              requireArgs("name-or-vmid"),
	ValidArgsFunction: completeVMNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		return runQmClone(client, args)
	},
}

// runQmClone mirrors runCtClone exactly, for QEMU VMs.
func runQmClone(client *api.Client, args []string) error {
	v, err := findVM(client, args[0])
	if err != nil {
		return err
	}

	newid := qmCloneNewID
	if newid == 0 {
		newid, err = client.NextID(context.Background())
		if err != nil {
			return fmt.Errorf("assigning next free vmid: %w", err)
		}
	}

	params := api.CloneVMParams{
		TargetVMID:  newid,
		Name:        qmCloneName,
		Storage:     qmCloneStorage,
		Full:        qmCloneFull,
		Target:      qmCloneTarget,
		Pool:        qmClonePool,
		Description: qmCloneDesc,
		SnapName:    qmCloneSnapname,
	}

	upid, err := client.CloneVM(context.Background(), v.Node, v.VMID, params)
	if err != nil {
		return fmt.Errorf("cloning %s (%d) to %d: %w", v.Name, v.VMID, newid, err)
	}
	return runProgressAction(client, v.Node, upid,
		fmt.Sprintf("cloning %s (%d) to %d", v.Name, v.VMID, newid),
		fmt.Sprintf("cloned %s (%d) to %d", v.Name, v.VMID, newid))
}

func init() {
	qmCloneCmd.Flags().IntVar(&qmCloneNewID, "newid", 0, "vmid for the clone (0 = auto-assign the next free ID)")
	qmCloneCmd.Flags().StringVar(&qmCloneName, "name", "", "name for the clone (defaults to the source's)")
	qmCloneCmd.Flags().StringVar(&qmCloneStorage, "storage", "", "target storage for a full clone (defaults to the source's storage)")
	qmCloneCmd.Flags().BoolVar(&qmCloneFull, "full", false, "force a full (independent) clone instead of a linked clone")
	qmCloneCmd.Flags().StringVar(&qmCloneTarget, "target", "", "node to create the clone on (defaults to the source's node)")
	qmCloneCmd.Flags().StringVar(&qmClonePool, "pool", "", "resource pool for the clone")
	qmCloneCmd.Flags().StringVar(&qmCloneDesc, "description", "", "description for the clone")
	qmCloneCmd.Flags().StringVar(&qmCloneSnapname, "snapname", "", "clone from this snapshot instead of the VM's current state")
	qmCmd.AddCommand(qmCloneCmd)
}
