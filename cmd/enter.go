package cmd

import (
	"github.com/davegallant/pvectl/internal/ssh"
	"github.com/spf13/cobra"
)

var enterCmd = &cobra.Command{
	Use:               "enter <name-or-vmid>",
	Short:             "Enter a container's shell over SSH",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeContainerNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := loadClient()
		if err != nil {
			return friendlySetupError(err)
		}
		c, err := resolveContainer(client, args)
		if err != nil {
			return err
		}
		return ssh.Enter(c.Node, c.VMID)
	},
}

func init() {
	ctCmd.AddCommand(enterCmd)
}
