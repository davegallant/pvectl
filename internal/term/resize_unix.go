//go:build !windows

package term

import (
	"os"
	"os/signal"
	"syscall"
)

// notifyResize watches SIGWINCH, the standard Unix terminal-resize signal —
// see resize_windows.go for the Windows side, which has no equivalent.
func notifyResize() (chan os.Signal, func()) {
	resizeCh := make(chan os.Signal, 1)
	signal.Notify(resizeCh, syscall.SIGWINCH)
	return resizeCh, func() { signal.Stop(resizeCh) }
}
