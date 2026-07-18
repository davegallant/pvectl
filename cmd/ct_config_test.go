package cmd

import (
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestConfigAppendCommandRegistered(t *testing.T) {
	if _, _, err := rootCmd.Find([]string{"ct", "config", "append"}); err != nil {
		t.Errorf("rootCmd.Find([ct config append]) error = %v", err)
	}
}

func TestRunAppendConfigNoLines(t *testing.T) {
	ctConfigAppendLines = nil
	c := api.Container{VMID: 101, Name: "web", Node: "pve1"}

	if err := runAppendConfig(nil, c); err == nil {
		t.Fatal("runAppendConfig() error = nil, want error for missing --line")
	}
}

func TestRunAppendConfigInvalidPrefix(t *testing.T) {
	ctConfigAppendLines = []string{"not.a.raw.line: foo"}
	defer func() { ctConfigAppendLines = nil }()
	c := api.Container{VMID: 101, Name: "web", Node: "pve1"}

	if err := runAppendConfig(nil, c); err == nil {
		t.Fatal("runAppendConfig() error = nil, want error for line not starting with \"lxc.\"")
	}
}
