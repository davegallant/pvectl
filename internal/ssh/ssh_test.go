package ssh

import (
	"context"
	"io"
	"reflect"
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

func TestBuildExecCmdArgs(t *testing.T) {
	cmd := buildExecCmd("pve1", 101, []string{"ls", "-la"})

	want := []string{"ssh", "pve1", "pct", "exec", "101", "--", "ls", "-la"}
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

func TestBuildListDirCmdArgs(t *testing.T) {
	cmd := buildListDirCmd(context.Background(), "pve1", 101, "sub/dir/")

	want := []string{"ssh", "-o", "BatchMode=yes", "pve1", "pct", "exec", "101", "--", "ls", "-1p", "--", "sub/dir/"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("Args = %v, want %v", cmd.Args, want)
	}
	for i, arg := range want {
		if cmd.Args[i] != arg {
			t.Errorf("Args[%d] = %q, want %q", i, cmd.Args[i], arg)
		}
	}
}

func TestParseListDirOutput(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []string
	}{
		{"empty", "", nil},
		{"single entry", "docker-compose.yml\n", []string{"docker-compose.yml"}},
		{"multiple entries with dir marker", "Dockerfile\ndocker-compose.yml\nsubdir/\n", []string{"Dockerfile", "docker-compose.yml", "subdir/"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseListDirOutput([]byte(tt.out))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseListDirOutput(%q) = %v, want %v", tt.out, got, tt.want)
			}
		})
	}
}
