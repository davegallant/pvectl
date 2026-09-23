package cmd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/spf13/cobra"
)

var configFieldName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
var vmVolumeField = regexp.MustCompile(`^(scsi|virtio|sata|ide|efidisk|tpmstate|unused)[0-9]+$`)
var ctVolumeField = regexp.MustCompile(`^(mp|unused|dev)[0-9]+$`)

func validateConfigField(guest, field string) error {
	if !configFieldName.MatchString(field) || field == "digest" || field == "delete" || field == "revert" || field == "force" || field == "skiplock" || field == "lxc" || strings.HasPrefix(field, "lxc.") {
		return fmt.Errorf("unsupported config field %q", field)
	}
	if guest == "qemu" && field == "sshkeys" {
		return fmt.Errorf("config field %q requires special Proxmox encoding; use qm create --sshkeys or the Proxmox UI", field)
	}
	if (guest == "lxc" && (field == "rootfs" || ctVolumeField.MatchString(field))) || (guest == "qemu" && (field == "vmstate" || vmVolumeField.MatchString(field))) {
		return fmt.Errorf("volume-backed field %q is not supported by config set/unset; use a dedicated command or the Proxmox UI", field)
	}
	return nil
}

func updateConfigField(client *api.Client, guest, node string, vmid int, action, field, value string) error {
	if err := validateConfigField(guest, field); err != nil {
		return err
	}
	ctx := context.Background()
	var fields map[string]string
	var digest string
	if guest == "lxc" {
		cfg, err := client.GetConfig(ctx, node, vmid)
		if err != nil {
			return fmt.Errorf("fetching config: %w", err)
		}
		fields, digest = cfg.Fields, cfg.Digest
	} else {
		cfg, err := client.GetVMConfig(ctx, node, vmid)
		if err != nil {
			return fmt.Errorf("fetching config: %w", err)
		}
		fields, digest = cfg.Fields, cfg.Digest
	}
	current, exists := fields[field]
	if (action == "set" && exists && current == value) || (action == "unset" && !exists) {
		fmt.Println("no changes")
		return nil
	}
	var err error
	if guest == "lxc" {
		if action == "set" {
			err = client.PutConfig(ctx, node, vmid, map[string]string{field: value}, digest)
		} else {
			err = client.DeleteConfigField(ctx, node, vmid, field, digest)
		}
	} else {
		if action == "set" {
			err = client.PutVMConfig(ctx, node, vmid, map[string]string{field: value}, digest)
		} else {
			err = client.DeleteVMConfigField(ctx, node, vmid, field, digest)
		}
	}
	if errors.Is(err, api.ErrDigestMismatch) {
		return fmt.Errorf("config changed elsewhere, retry the command")
	}
	if err != nil {
		return fmt.Errorf("updating config: %w", err)
	}
	if action == "set" {
		fmt.Printf("set %s on %s %d\n", field, guest, vmid)
	} else {
		fmt.Printf("removed %s from %s %d\n", field, guest, vmid)
	}
	return nil
}

func newConfigUpdateCmd(guest, action string) *cobra.Command {
	argsNeeded := 2
	use := action + " <name-or-vmid> <field>"
	if action == "set" {
		argsNeeded = 3
		use += " <value>"
	}
	cmd := &cobra.Command{
		Use:         use,
		Short:       strings.ToUpper(action[:1]) + action[1:] + " a regular config field with digest protection",
		Annotations: mutationAnnotation(mutationMutating),
		Args:        cobra.ExactArgs(argsNeeded),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := validateConfigField(guest, args[1]); err != nil {
				return err
			}
			client, err := loadClient()
			if err != nil {
				return friendlySetupError(err)
			}
			var node string
			var vmid int
			if guest == "lxc" {
				c, err := resolveContainer(client, args[:1])
				if err != nil {
					return err
				}
				node, vmid = c.Node, c.VMID
			} else {
				v, err := resolveVM(client, args[:1])
				if err != nil {
					return err
				}
				node, vmid = v.Node, v.VMID
			}
			value := ""
			if action == "set" {
				value = args[2]
			}
			return updateConfigField(client, guest, node, vmid, action, args[1], value)
		},
	}
	if guest == "lxc" {
		cmd.ValidArgsFunction = completeContainerNames
	} else {
		cmd.ValidArgsFunction = completeVMNames
	}
	return cmd
}

func init() {
	for _, action := range []string{"set", "unset"} {
		ctConfigCmd.AddCommand(newConfigUpdateCmd("lxc", action))
		qmConfigCmd.AddCommand(newConfigUpdateCmd("qemu", action))
	}
}
