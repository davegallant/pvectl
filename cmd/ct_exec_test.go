package cmd

import "testing"

func TestExecCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"ct", "exec"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("ct", "exec") error = %v`, err)
	}
	if found.Name() != "exec" {
		t.Errorf(`Find("ct", "exec").Name() = %q, want "exec"`, found.Name())
	}
}
