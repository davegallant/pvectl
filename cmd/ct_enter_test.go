package cmd

import "testing"

func TestEnterCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"ct", "enter"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("ct", "enter") error = %v`, err)
	}
	if found.Name() != "enter" {
		t.Errorf(`Find("ct", "enter").Name() = %q, want "enter"`, found.Name())
	}
}
