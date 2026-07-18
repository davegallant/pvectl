package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/davegallant/pvectl/internal/editconf"
	"github.com/davegallant/pvectl/internal/tui"
)

// selectContainer fetches the container list from the cluster and runs the
// interactive picker, returning the chosen container.
func selectContainer(client *api.Client) (api.Container, error) {
	containers, err := client.ListContainers(context.Background())
	if err != nil {
		return api.Container{}, fmt.Errorf("listing containers: %w", err)
	}

	fetch := func(node string, vmid int) (string, error) {
		cfg, err := client.GetConfig(context.Background(), node, vmid)
		if err != nil {
			return "", err
		}
		return editconf.RenderPreview(cfg.Fields) + cfg.RawLXC, nil
	}

	return tui.RunPicker(containers, fetch)
}

// findContainer looks up a container by exact vmid or exact name, for
// commands given a name-or-vmid argument directly instead of using the
// interactive picker. A numeric identifier is matched against vmid
// (unique cluster-wide); anything else is matched against name (not
// guaranteed unique, so an ambiguous match is rejected rather than
// guessed at).
func findContainer(client *api.Client, identifier string) (api.Container, error) {
	containers, err := client.ListContainers(context.Background())
	if err != nil {
		return api.Container{}, fmt.Errorf("listing containers: %w", err)
	}

	if vmid, err := strconv.Atoi(identifier); err == nil {
		for _, c := range containers {
			if c.VMID == vmid {
				return c, nil
			}
		}
		return api.Container{}, fmt.Errorf("no container with vmid %d found", vmid)
	}

	var matches []api.Container
	for _, c := range containers {
		if c.Name == identifier {
			matches = append(matches, c)
		}
	}
	switch len(matches) {
	case 0:
		return api.Container{}, fmt.Errorf("no container named %q found", identifier)
	case 1:
		return matches[0], nil
	default:
		return api.Container{}, fmt.Errorf("multiple containers named %q found, use its vmid instead", identifier)
	}
}

// resolveContainer returns the container named/vmid'd by args[0] when
// given, skipping the interactive picker entirely — the shared mechanism
// behind every `ct` command's "pvectl ct <action> <name-or-vmid>" form
// (it already knows what was selected, so it shouldn't ask again), first
// established for `ct migrate` and then extended to every other action.
// Falls back to the fuzzy picker when args is empty.
func resolveContainer(client *api.Client, args []string) (api.Container, error) {
	if len(args) == 0 {
		return selectContainer(client)
	}
	return findContainer(client, args[0])
}
