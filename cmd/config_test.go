package cmd

import (
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/config"
)

func TestConfigViewCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"config", "view"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("config", "view") error = %v`, err)
	}
	if found.Use != "view" {
		t.Errorf(`Find("config", "view").Use = %q, want "view"`, found.Use)
	}
}

func TestRenderConfigShowsFieldsButNeverASecret(t *testing.T) {
	cfg := &config.Config{
		Host:               "https://pve.example.com:8006",
		TokenID:            "user@pve!pvectl",
		InsecureSkipVerify: true,
		SecretBackend:      "file",
		ConsoleMethod:      "api",
	}

	got, err := renderConfig(cfg)
	if err != nil {
		t.Fatalf("renderConfig() error = %v", err)
	}

	for _, want := range []string{cfg.Host, cfg.TokenID, "true", "file", "api"} {
		if !strings.Contains(got, want) {
			t.Errorf("renderConfig() = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "secret") == false {
		t.Errorf("renderConfig() = %q, want it to contain the secret_backend key", got)
	}
}

func TestRenderConfigOmitsEmptyOptionalFields(t *testing.T) {
	cfg := &config.Config{
		Host:    "https://pve.example.com:8006",
		TokenID: "user@pve!pvectl",
	}

	got, err := renderConfig(cfg)
	if err != nil {
		t.Fatalf("renderConfig() error = %v", err)
	}

	for _, notWant := range []string{"secret_backend", "console_method"} {
		if strings.Contains(got, notWant) {
			t.Errorf("renderConfig() = %q, want it to omit empty %q", got, notWant)
		}
	}
}
