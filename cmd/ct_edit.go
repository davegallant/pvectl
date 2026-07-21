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

var ctEditCmd = &cobra.Command{
	Use:               "edit <name-or-vmid>",
	Short:             "Edit a container's config in $EDITOR",
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
		return runEdit(client, c.Node, c.VMID)
	},
}

func init() {
	ctConfigCmd.AddCommand(ctEditCmd)
}

func runEdit(client *api.Client, node string, vmid int) error {
	ctx := context.Background()

	cfg, err := client.GetConfig(ctx, node, vmid)
	if err != nil {
		return fmt.Errorf("fetching config: %w", err)
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("pvectl-edit-%d-*.conf", vmid))
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.WriteString(editconf.Render(cfg.Fields) + cfg.RawLXC); err != nil {
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

	if err := applyEdit(client, node, vmid, cfg, string(edited)); err != nil {
		return fmt.Errorf("%w (config left at %s)", err, tmpPath)
	}

	_ = os.Remove(tmpPath)
	return nil
}

// applyEdit diffs the edited config text against the original fields and,
// if anything changed, PUTs the changed fields with the original digest.
func applyEdit(client *api.Client, node string, vmid int, original api.Config, editedText string) error {
	edited := editconf.Parse(editedText)
	// "lxc.*" lines are shown in the editor for context (rendered from
	// RawLXC, which has no dedicated Proxmox API parameter and can't
	// round-trip through a map — see api.Config.RawLXC) but are not
	// currently editable. Strip them before diffing so they're never
	// sent to PutConfig, regardless of whether the user touched them.
	for k := range edited {
		if strings.HasPrefix(k, "lxc.") {
			delete(edited, k)
		}
	}

	diff := editconf.DiffFields(original.Fields, edited)
	if len(diff.Removed) > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d field(s) removed in editor are not supported and were ignored: %s\n", len(diff.Removed), strings.Join(diff.Removed, ", "))
	}
	if len(diff.Changed) == 0 {
		fmt.Println("no changes")
		return nil
	}

	err := client.PutConfig(context.Background(), node, vmid, diff.Changed, original.Digest)
	if errors.Is(err, api.ErrDigestMismatch) {
		return fmt.Errorf("config changed elsewhere, re-run edit")
	}
	if err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("updated %d field(s)\n", len(diff.Changed))
	return nil
}
