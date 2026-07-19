package cmd

import (
	"strings"
	"testing"

	"github.com/davegallant/pvectl/internal/api"
)

func TestEnterConsoleRejectsInvalidMethodOverride(t *testing.T) {
	sshCalled := false
	sshEnter := func(node string, vmid int) error {
		sshCalled = true
		return nil
	}

	err := enterConsole(nil, "pve1", 101, api.KindContainer, sshEnter, "telnet")
	if err == nil {
		t.Fatal("enterConsole() error = nil, want error for invalid --method")
	}
	if !strings.Contains(err.Error(), "telnet") {
		t.Errorf("enterConsole() error = %q, want it to mention the invalid value", err.Error())
	}
	if sshCalled {
		t.Error("sshEnter was called despite invalid --method — should have returned before dispatching")
	}
}

func TestEnterConsoleMethodOverrideSSH(t *testing.T) {
	sshCalled := false
	sshEnter := func(node string, vmid int) error {
		sshCalled = true
		if node != "pve1" || vmid != 101 {
			t.Errorf("sshEnter(%q, %d), want (\"pve1\", 101)", node, vmid)
		}
		return nil
	}

	if err := enterConsole(nil, "pve1", 101, api.KindContainer, sshEnter, "ssh"); err != nil {
		t.Fatalf("enterConsole() error = %v", err)
	}
	if !sshCalled {
		t.Error("sshEnter was not called for --method=ssh")
	}
}
