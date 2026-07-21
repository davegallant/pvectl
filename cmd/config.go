package cmd

import (
	"fmt"

	"github.com/davegallant/pvectl/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage pvectl's own configuration",
}

var configViewCmd = &cobra.Command{
	Use:         "view",
	Short:       "Show pvectl's stored configuration",
	Annotations: mutationAnnotation(mutationSafe),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return friendlySetupError(err)
		}
		out, err := renderConfig(cfg)
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configViewCmd)
	rootCmd.AddCommand(configCmd)
}

// renderConfig formats cfg as YAML, the same shape it's stored on disk in
// (config.Save also uses yaml.Marshal). The token secret is never
// included: it isn't part of config.Config at all — it lives in the OS
// keyring (or file-store fallback) instead, keyed by host, and is never
// loaded into this struct.
func renderConfig(cfg *config.Config) (string, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("encoding config: %w", err)
	}
	return string(data), nil
}
