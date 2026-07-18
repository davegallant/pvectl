package ssh

import (
	"io"
	"testing"
)

func TestBuildCmdArgs(t *testing.T) {
	cmd := buildCmd("pve1", 101)

	want := []string{"ssh", "-t", "pve1", "pct", "enter", "101"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("Args = %v, want %v", cmd.Args, want)
	}
	for i, arg := range want {
		if cmd.Args[i] != arg {
			t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], arg)
		}
	}
}

func TestBuildVMCmdArgs(t *testing.T) {
	cmd := buildVMCmd("pve1", 201)

	want := []string{"ssh", "-t", "pve1", "qm", "terminal", "201"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("Args = %v, want %v", cmd.Args, want)
	}
	for i, arg := range want {
		if cmd.Args[i] != arg {
			t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], arg)
		}
	}
}

func TestBuildAppendRawConfigCmdArgs(t *testing.T) {
	cmd := buildAppendRawConfigCmd("pve1", 101, []string{
		"lxc.cgroup2.devices.allow: c 10:200 rwm",
		"lxc.mount.entry: /dev/net dev/net none bind,create=dir",
	})

	want := []string{"ssh", "pve1", "cat", ">>", "/etc/pve/lxc/101.conf"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("Args = %v, want %v", cmd.Args, want)
	}
	for i, arg := range want {
		if cmd.Args[i] != arg {
			t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], arg)
		}
	}

	stdin, err := io.ReadAll(cmd.Stdin)
	if err != nil {
		t.Fatalf("reading cmd.Stdin: %v", err)
	}
	want2 := "lxc.cgroup2.devices.allow: c 10:200 rwm\nlxc.mount.entry: /dev/net dev/net none bind,create=dir\n"
	if string(stdin) != want2 {
		t.Errorf("stdin = %q, want %q", string(stdin), want2)
	}
}
