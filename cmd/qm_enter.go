package cmd

import (
	"github.com/davegallant/pvectl/internal/ssh"
	"github.com/spf13/cobra"
)

var qmEnterCmd = &cobra.Command{
	Use:               "enter <name-or-vmid>",
	Short:             "Attach to a VM's serial console over SSH",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeVMNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		v, err := resolveVM(client, args)
		if err != nil {
			return err
		}
		return ssh.EnterVM(v.Node, v.VMID)
	},
}

func init() {
	qmCmd.AddCommand(qmEnterCmd)
}
