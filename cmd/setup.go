package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/davegallant/pvectl/internal/api"
	"github.com/davegallant/pvectl/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var setupInsecureSkipVerify bool

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Store Proxmox API credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		existing, err := config.Load()
		if err != nil {
			if !errors.Is(err, config.ErrNotFound) {
				return fmt.Errorf("loading existing config: %w", err)
			}
			existing = nil
		}

		reader := bufio.NewReader(os.Stdin)

		var existingHost, existingTokenID string
		if existing != nil {
			existingHost, existingTokenID = existing.Host, existing.TokenID
		}

		host := promptWithDefault(reader, "Proxmox host", existingHost, "https://pve.example.com:8006")
		tokenID := promptWithDefault(reader, "Token ID", existingTokenID, "user@realm!tokenid")

		if existing != nil {
			fmt.Print("Token secret (leave blank to keep existing): ")
		} else {
			fmt.Print("Token secret: ")
		}
		secretBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("reading token secret: %w", err)
		}
		secret := strings.TrimSpace(string(secretBytes))

		if secret == "" {
			if existing == nil {
				return fmt.Errorf("token secret is required")
			}
			existingSecret, err := secretStoreFor(existing).Get(existing.Host)
			if err != nil {
				return fmt.Errorf("reading existing token secret: %w", err)
			}
			secret = existingSecret
		}

		insecureSkipVerify := setupInsecureSkipVerify
		if !cmd.Flags().Changed("insecure-skip-verify") && existing != nil {
			insecureSkipVerify = existing.InsecureSkipVerify
		}

		return runSetup(host, tokenID, secret, insecureSkipVerify)
	},
}

// promptWithDefault prints label followed by a bracketed hint, reads a line
// from reader, and returns the trimmed input. The hint is defaultValue when
// non-empty — in which case blank input returns defaultValue unchanged — or
// otherwise example, shown only to illustrate the expected format; blank
// input in that case returns "".
func promptWithDefault(reader *bufio.Reader, label, defaultValue, example string) string {
	hint := defaultValue
	if hint == "" {
		hint = example
	}
	if hint != "" {
		fmt.Printf("%s [%s]: ", label, hint)
	} else {
		fmt.Printf("%s: ", label)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

func init() {
	setupCmd.Flags().BoolVar(&setupInsecureSkipVerify, "insecure-skip-verify", false, "skip TLS certificate verification (self-signed clusters)")
	rootCmd.AddCommand(setupCmd)
}

// runSetup validates the given credentials against the Proxmox API and, on
// success, persists them via config.Save and the OS keychain (falling back
// to a local file if the keychain is unavailable — e.g. no D-Bus Secret
// Service running).
func runSetup(host, tokenID, secret string, insecureSkipVerify bool) error {
	client := api.NewClient(host, tokenID, secret, insecureSkipVerify)
	client.SetDebug(debug)

	if _, err := client.Version(context.Background()); err != nil {
		return fmt.Errorf("validating credentials: %w", err)
	}

	backend := "keyring"
	if err := keyringStore.Set(host, secret); err != nil {
		fmt.Fprintf(os.Stderr, "warning: OS keychain unavailable (%v); falling back to file-based secret storage\n", err)
		if fileErr := fileStore.Set(host, secret); fileErr != nil {
			return fmt.Errorf("saving secret (keyring and file fallback both failed): keyring: %v, file: %w", err, fileErr)
		}
		backend = "file"
	}

	cfg := &config.Config{
		Host:               host,
		TokenID:            tokenID,
		InsecureSkipVerify: insecureSkipVerify,
		SecretBackend:      backend,
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println("Setup complete.")
	return nil
}
