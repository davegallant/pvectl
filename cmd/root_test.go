package cmd

import "testing"

func TestRootCommandConfiguration(t *testing.T) {
	if rootCmd.Use != "pvectl" {
		t.Errorf("rootCmd.Use = %q, want %q", rootCmd.Use, "pvectl")
	}
	if !rootCmd.SilenceUsage {
		t.Error("rootCmd.SilenceUsage should be true")
	}
	if !rootCmd.SilenceErrors {
		t.Error("rootCmd.SilenceErrors should be true (main.go owns error printing)")
	}
}
