package cmd

import (
	"github.com/davegallant/pvectl/internal/api"
	"github.com/davegallant/pvectl/internal/ssh"
	"github.com/spf13/cobra"
)

var qmEnterMethod string

var qmEnterCmd = &cobra.Command{
	Use:               "enter <name-or-vmid>",
	Short:             "Attach to a VM's serial console via SSH (see --method for the API alternative)",
	Annotations:       mutationAnnotation(mutationDestructive),
	Args:              requireArgs("name-or-vmid"),
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
		return enterConsole(client, v.Node, v.VMID, api.KindVM, ssh.EnterVM, qmEnterMethod)
	},
}

func init() {
	qmEnterCmd.Flags().StringVar(&qmEnterMethod, "method", "", `override the configured console method for this run ("ssh" or "api")`)
	qmCmd.AddCommand(qmEnterCmd)
}
