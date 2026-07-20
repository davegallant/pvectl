//go:build windows

package term

import "os"

// notifyResize has no signal to watch on Windows — there's no SIGWINCH
// equivalent, so the returned channel never fires and the remote side
// only ever learns the terminal size once, from Attach's initial
// sendResize call.
func notifyResize() (chan os.Signal, func()) {
	return make(chan os.Signal), func() {}
}
