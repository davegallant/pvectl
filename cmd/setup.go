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
	Use:         "setup",
	Short:       "Store Proxmox API credentials",
	Annotations: mutationAnnotation(mutationMutating),
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
		existingAPIConsole := false
		if existing != nil {
			existingHost, existingTokenID = existing.Host, existing.TokenID
			existingAPIConsole = existing.ConsoleMethod == "api"
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

		useAPIConsole := promptYesNoDefault(reader, "Enable API-based console access (skip SSH for ct/qm enter)?", existingAPIConsole)

		return runSetup(host, tokenID, secret, insecureSkipVerify, useAPIConsole)
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

// promptYesNoDefault prints label with a [y/N] or [Y/n] hint reflecting
// defaultValue, reads a line from reader, and returns the parsed answer.
// Blank input returns defaultValue unchanged; any input starting with "y"
// or "n" (case-insensitive) returns true/false regardless of default.
// Distinct from promptYesNo (ct_create.go): that one always reads from
// os.Stdin directly and has no default, which doesn't fit this prompt's
// need for a reader-testable, default-aware yes/no.
func promptYesNoDefault(reader *bufio.Reader, label string, defaultValue bool) bool {
	hint := "y/N"
	if defaultValue {
		hint = "Y/n"
	}
	fmt.Printf("%s [%s]: ", label, hint)
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	switch {
	case input == "":
		return defaultValue
	case strings.HasPrefix(input, "y"):
		return true
	case strings.HasPrefix(input, "n"):
		return false
	default:
		return defaultValue
	}
}

func init() {
	setupCmd.Flags().BoolVar(&setupInsecureSkipVerify, "insecure-skip-verify", false, "skip TLS certificate verification (self-signed clusters)")
	rootCmd.AddCommand(setupCmd)
}

// runSetup validates the given credentials against the Proxmox API and, on
// success, persists them via config.Save and the OS keychain (falling back
// to a local file if the keychain is unavailable — e.g. no D-Bus Secret
// Service running).
func runSetup(host, tokenID, secret string, insecureSkipVerify, useAPIConsole bool) error {
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

	consoleMethod := ""
	if useAPIConsole {
		consoleMethod = "api"
	}

	cfg := &config.Config{
		Host:               host,
		TokenID:            tokenID,
		InsecureSkipVerify: insecureSkipVerify,
		SecretBackend:      backend,
		ConsoleMethod:      consoleMethod,
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println("Setup complete.")
	return nil
}
