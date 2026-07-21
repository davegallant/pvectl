package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/davegallant/pvectl/internal/editconf"
	"github.com/spf13/cobra"
)

var qmEditCmd = &cobra.Command{
	Use:               "edit <name-or-vmid>",
	Short:             "Edit a VM's config in $EDITOR",
	Annotations:       mutationAnnotation(mutationMutating),
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
		return runEditVM(client, v.Node, v.VMID)
	},
}

func init() {
	qmConfigCmd.AddCommand(qmEditCmd)
}

func runEditVM(client *api.Client, node string, vmid int) error {
	ctx := context.Background()

	cfg, err := client.GetVMConfig(ctx, node, vmid)
	if err != nil {
		return fmt.Errorf("fetching config: %w", err)
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("pvectl-qm-edit-%d-*.conf", vmid))
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(editconf.Render(cfg.Fields)); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	_ = tmpFile.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	editorCmd := exec.Command(editor, tmpPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr
	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("running editor (config left at %s): %w", tmpPath, err)
	}

	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("reading edited config (left at %s): %w", tmpPath, err)
	}

	if err := applyVMEdit(client, node, vmid, cfg, string(edited)); err != nil {
		return fmt.Errorf("%w (config left at %s)", err, tmpPath)
	}

	_ = os.Remove(tmpPath)
	return nil
}

// applyVMEdit diffs the edited config text against the original fields
// and, if anything changed, PUTs the changed fields with the original
// digest. Unlike applyEdit (LXC), there's no "lxc.*" passthrough block to
// strip before diffing — VMConfig has no such lines to begin with.
func applyVMEdit(client *api.Client, node string, vmid int, original api.VMConfig, editedText string) error {
	edited := editconf.Parse(editedText)

	diff := editconf.DiffFields(original.Fields, edited)
	if len(diff.Removed) > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d field(s) removed in editor are not supported and were ignored: %s\n", len(diff.Removed), strings.Join(diff.Removed, ", "))
	}
	if len(diff.Changed) == 0 {
		fmt.Println("no changes")
		return nil
	}

	err := client.PutVMConfig(context.Background(), node, vmid, diff.Changed, original.Digest)
	if errors.Is(err, api.ErrDigestMismatch) {
		return fmt.Errorf("config changed elsewhere, re-run edit")
	}
	if err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("updated %d field(s)\n", len(diff.Changed))
	return nil
}
