package cmd

import "testing"

func TestQmEnterCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"qm", "enter"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("qm", "enter") error = %v`, err)
	}
	if found.Name() != "enter" {
		t.Errorf(`Find("qm", "enter").Name() = %q, want "enter"`, found.Name())
	}
}
