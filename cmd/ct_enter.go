package cmd

import (
	"github.com/davegallant/pvectl/internal/api"
	"github.com/davegallant/pvectl/internal/ssh"
	"github.com/spf13/cobra"
)

var ctEnterMethod string

var ctEnterCmd = &cobra.Command{
	Use:               "enter <name-or-vmid>",
	Short:             "Enter a container's shell via SSH (see --method for the API alternative)",
	Annotations:       mutationAnnotation(mutationDestructive),
	Args:              requireArgs("name-or-vmid"),
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
		return enterConsole(client, c.Node, c.VMID, api.KindContainer, ssh.Enter, ctEnterMethod)
	},
}

func init() {
	ctEnterCmd.Flags().StringVar(&ctEnterMethod, "method", "", `override the configured console method for this run ("ssh" or "api")`)
	ctCmd.AddCommand(ctEnterCmd)
}
