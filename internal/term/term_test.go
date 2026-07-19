package term

import "testing"

func TestEscapeStateFeed(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOutput string
		wantDetach bool
	}{
		{"plain text passes through unchanged", "ls -la\r", "ls -la\r", false},
		{"tilde mid-command is not an escape", "echo ~/foo\r", "echo ~/foo\r", false},
		{"~. at start of session detaches immediately", "~.", "", true},
		{"~. after a newline detaches", "ls\r~.", "ls\r", true},
		{"~~ sends one literal tilde", "~~\r", "~\r", false},
		{"tilde followed by unrecognized char forwards both", "~x", "~x", false},
		{"trailing lone tilde is buffered, not forwarded", "ls\r~", "ls\r", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newEscapeState()
			var out []byte
			detached := false
			for i := 0; i < len(tt.input); i++ {
				forward, detach := s.feed(tt.input[i])
				out = append(out, forward...)
				if detach {
					detached = true
					break
				}
			}
			if string(out) != tt.wantOutput {
				t.Errorf("forwarded output = %q, want %q", out, tt.wantOutput)
			}
			if detached != tt.wantDetach {
				t.Errorf("detach = %v, want %v", detached, tt.wantDetach)
			}
		})
	}
}

func TestEscapeStateAcrossChunkBoundary(t *testing.T) {
	// The "~" and "." can arrive in separate stdin Read() calls — the
	// state must persist across feed() calls to still catch this.
	s := newEscapeState()

	forward, detach := s.feed('~')
	if len(forward) != 0 || detach {
		t.Fatalf("feed('~') = (%q, %v), want (\"\", false)", forward, detach)
	}

	forward, detach = s.feed('.')
	if len(forward) != 0 || !detach {
		t.Fatalf("feed('.') = (%q, %v), want (\"\", true)", forward, detach)
	}
}

func TestEscapeStateOnlyAtLineStart(t *testing.T) {
	// A "~" that isn't at the start of a line is just a literal
	// character — typing "ls ~." (a real, if unusual, shell argument)
	// must not be treated as an escape.
	s := newEscapeState()
	var out []byte
	for _, b := range []byte("ls ~.\r") {
		forward, detach := s.feed(b)
		out = append(out, forward...)
		if detach {
			t.Fatalf("unexpected detach mid-command at byte %q", b)
		}
	}
	if string(out) != "ls ~.\r" {
		t.Errorf("forwarded output = %q, want %q", out, "ls ~.\r")
	}
}
