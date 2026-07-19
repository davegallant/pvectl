package cmd

import (
	"reflect"
	"testing"
)

func TestExecCommandRegistered(t *testing.T) {
	found, _, err := rootCmd.Find([]string{"ct", "exec"})
	if err != nil {
		t.Fatalf(`rootCmd.Find("ct", "exec") error = %v`, err)
	}
	if found.Name() != "exec" {
		t.Errorf(`Find("ct", "exec").Name() = %q, want "exec"`, found.Name())
	}
}

func TestExecCandidates(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		dir     string
		prefix  string
		want    []string
	}{
		{
			name:    "no dir, filters by prefix",
			entries: []string{"docker-compose.yml", "docker-compose.override.yml", "Dockerfile", "README.md"},
			dir:     "",
			prefix:  "docker-comp",
			want:    []string{"docker-compose.yml", "docker-compose.override.yml"},
		},
		{
			name:    "dir reattached to matches",
			entries: []string{"file.txt", "other.txt"},
			dir:     "sub/dir/",
			prefix:  "file",
			want:    []string{"sub/dir/file.txt"},
		},
		{
			name:    "empty prefix matches everything",
			entries: []string{"a", "b"},
			dir:     "",
			prefix:  "",
			want:    []string{"a", "b"},
		},
		{
			name:    "no matches",
			entries: []string{"a", "b"},
			dir:     "",
			prefix:  "z",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := execCandidates(tt.entries, tt.dir, tt.prefix)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("execCandidates(%v, %q, %q) = %v, want %v", tt.entries, tt.dir, tt.prefix, got, tt.want)
			}
		})
	}
}
