// Package term attaches the local terminal to a Proxmox console websocket
// (see api.Client.OpenConsole), as an alternative to internal/ssh's
// `ssh -t node pct/qm ...` path that needs only an API token, not SSH
// access to the node.
package term

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/coder/websocket"
	xterm "golang.org/x/term"
)

// pingInterval keeps the console websocket alive well inside the 5-minute
// idle timeout pve-xtermjs's termproxy protocol documents
// (https://github.com/proxmox/pve-xtermjs).
const pingInterval = time.Minute

// errDetach is pumpInput's sentinel for "the user typed the ~. escape" —
// caught in Attach to end the session cleanly (no error message), rather
// than treated as a connection failure.
var errDetach = errors.New("detached")

// Attach puts the local terminal (stdin/stdout, assumed to be a real tty)
// into raw mode and pumps bytes between it and conn until either side
// closes (or the user detaches via "~.", see pumpInput), speaking
// Proxmox's termproxy framing protocol: client->server messages are
// framed as "0:LEN:MSG" (terminal input) or "1:COLS:ROWS:" (resize), plus
// a bare "2" ping to hold the connection open. Server->client is raw
// passthrough with no framing at all — whatever bytes arrive are written
// straight to stdout. Restores the terminal and closes conn before
// returning.
func Attach(ctx context.Context, conn *websocket.Conn) error {
	fd := int(os.Stdin.Fd())
	oldState, err := xterm.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("entering raw terminal mode: %w", err)
	}
	defer func() { _ = xterm.Restore(fd, oldState) }()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resizeCh, stopResize := notifyResize()
	defer stopResize()

	// \r\n, not \n: MakeRaw disabled output post-processing, so a bare \n
	// here would move down a line without returning to column 0.
	fmt.Print("Attached. Press Enter to continue. Type ~. to detach.\r\n")

	finish := func(err error) error {
		cancel()
		_ = conn.Close(websocket.StatusNormalClosure, "")
		if errors.Is(err, errDetach) {
			return nil
		}
		return err
	}

	inputErr := make(chan error, 1)
	inputReady := make(chan struct{})
	// pumpInput's os.Stdin.Read blocks regardless of ctx cancellation, so
	// this goroutine may outlive Attach when the server side closes
	// first — harmless, since it exits with the process either way.
	go func() { inputErr <- pumpInput(ctx, conn, inputReady) }()

	// Deliberately don't start pumpOutput or send the initial resize
	// until the user's first real keystroke: LXC's tty1 console is
	// reached via `dtach -A ... -r winch`, and dtach forces a screen
	// redraw of whatever's already running the instant it attaches —
	// entirely server-side, independent of anything sent from here. That
	// redraw was racing (and always winning against) the "Attached..."
	// message above, wiping it before it could be read, regardless of
	// how it was worded or when a resize/nudge was sent. Holding the
	// output side back until inputReady closes (pumpInput signals this
	// on its first stdin read, whatever the byte is — see pumpInput)
	// guarantees the message is the only thing on screen until the user
	// acts, which is the only way to actually win that race rather than
	// just narrow it.
	select {
	case <-inputReady:
	case runErr := <-inputErr:
		return finish(runErr)
	}

	sendResize(ctx, conn, fd)
	go watchResize(ctx, conn, fd, resizeCh)
	go ping(ctx, conn)

	outputErr := make(chan error, 1)
	go func() { outputErr <- pumpOutput(conn) }()

	var runErr error
	select {
	case runErr = <-outputErr:
	case runErr = <-inputErr:
	}
	return finish(runErr)
}

// pumpOutput copies conn's raw, unframed server->client bytes straight to
// stdout until conn closes. A normal close handshake (the server ending
// the session — e.g. `exit` in the LXC shell) surfaces here as a
// CloseError and is treated as a clean end of session, not a failure.
func pumpOutput(conn *websocket.Conn) error {
	for {
		_, data, err := conn.Read(context.Background())
		if err != nil {
			if websocket.CloseStatus(err) != -1 {
				return nil
			}
			return fmt.Errorf("reading console output: %w", err)
		}
		if _, err := os.Stdout.Write(data); err != nil {
			return fmt.Errorf("writing console output: %w", err)
		}
	}
}

// pumpInput reads raw keystrokes from stdin, recognizes the ssh-style
// "~." local detach escape via escapeState, and frames everything else as
// termproxy "0:LEN:MSG" data messages. Closes ready the first time
// os.Stdin.Read returns any bytes at all (whatever they are — Attach
// uses this purely as a "the user did something" signal, not to inspect
// content), exactly once. Returns errDetach if the user typed the
// escape, or nil on stdin EOF (e.g. the local terminal closing) rather
// than treating either as an error.
func pumpInput(ctx context.Context, conn *websocket.Conn, ready chan<- struct{}) error {
	buf := make([]byte, 4096)
	state := newEscapeState()
	firstRead := true

	for {
		n, readErr := os.Stdin.Read(buf)
		if n > 0 {
			if firstRead {
				firstRead = false
				close(ready)
			}
			out := make([]byte, 0, n)
			for i := 0; i < n; i++ {
				forward, detach := state.feed(buf[i])
				out = append(out, forward...)
				if detach {
					if len(out) > 0 {
						if err := writeInput(ctx, conn, out); err != nil {
							return err
						}
					}
					return errDetach
				}
			}
			if len(out) > 0 {
				if err := writeInput(ctx, conn, out); err != nil {
					return err
				}
			}
		}
		if readErr != nil {
			return nil
		}
	}
}

func writeInput(ctx context.Context, conn *websocket.Conn, data []byte) error {
	frame := fmt.Sprintf("0:%d:%s", len(data), data)
	if err := conn.Write(ctx, websocket.MessageText, []byte(frame)); err != nil {
		return fmt.Errorf("writing console input: %w", err)
	}
	return nil
}

// escapeState recognizes the ssh/rlogin/tip-style "~." local detach
// escape across a stream of bytes, tracking the two bits of state needed
// (are we at the start of a line, and is a "~" waiting to see its next
// byte) so the sequence is still caught even if it straddles two
// separate stdin Read() calls.
type escapeState struct {
	atLineStart  bool
	pendingTilde bool
}

// newEscapeState starts atLineStart true, matching ssh: the escape is
// recognized as the very first bytes of the session too, not just after
// a newline — useful for bailing out immediately if the remote side
// never responds at all.
func newEscapeState() *escapeState {
	return &escapeState{atLineStart: true}
}

// feed processes one input byte, returning the bytes (zero, one, or two)
// that should be forwarded to the remote, and whether this byte completed
// the "~." detach escape (in which case forward excludes the escape
// itself and the caller should stop reading).
func (s *escapeState) feed(b byte) (forward []byte, detach bool) {
	if s.pendingTilde {
		s.pendingTilde = false
		switch b {
		case '.':
			return nil, true
		case '~':
			// "~~" sends one literal tilde.
			s.atLineStart = false
			return []byte{'~'}, false
		default:
			// Not a recognized escape — the buffered tilde wasn't an
			// escape after all, so forward it along with b.
			s.atLineStart = b == '\r' || b == '\n'
			return []byte{'~', b}, false
		}
	}
	if s.atLineStart && b == '~' {
		s.pendingTilde = true
		return nil, false
	}
	s.atLineStart = b == '\r' || b == '\n'
	return []byte{b}, false
}

// sendResize looks up the local terminal's current size and sends it as a
// termproxy "1:COLS:ROWS:" message. Best-effort: a size query failure
// (e.g. stdin isn't actually a tty, unlikely given MakeRaw already
// succeeded) just leaves the remote size stale rather than failing the
// whole session.
func sendResize(ctx context.Context, conn *websocket.Conn, fd int) {
	cols, rows, err := xterm.GetSize(fd)
	if err != nil {
		return
	}
	frame := fmt.Sprintf("1:%d:%d:", cols, rows)
	_ = conn.Write(ctx, websocket.MessageText, []byte(frame))
}

// watchResize re-sends the terminal size on every signal from resizeCh
// (SIGWINCH on Unix; never, on Windows — see notifyResize) until ctx is
// cancelled.
func watchResize(ctx context.Context, conn *websocket.Conn, fd int, resizeCh <-chan os.Signal) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-resizeCh:
			sendResize(ctx, conn, fd)
		}
	}
}

// ping sends a termproxy keepalive ("2") on pingInterval until ctx is
// cancelled.
func ping(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = conn.Write(ctx, websocket.MessageText, []byte("2"))
		}
	}
}
