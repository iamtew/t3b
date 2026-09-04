// Package daemon will background the bot and route CLI commands into it.
// This pass only stubs the seams described in README ( -daemon , status, restart ).
package daemon

import (
	"fmt"
	"os"
)

// ErrNotImplemented is returned by stub entrypoints until IPC exists.
var ErrNotImplemented = fmt.Errorf("daemon mode is not implemented yet")

// Start is the future -daemon path. It must not background the process today.
func Start() error {
	fmt.Fprintln(os.Stderr, "t3b: -daemon is not implemented yet (foreground only for now)")
	return ErrNotImplemented
}

// Dispatch handles CLI router commands aimed at a running daemonized instance
// (e.g. "t3b status", "t3b restart"). No socket/IPC yet.
func Dispatch(args []string) error {
	cmd := "status"
	if len(args) > 0 {
		cmd = args[0]
	}
	fmt.Fprintf(os.Stderr, "t3b: command %q needs a daemonized instance; daemon IPC is not implemented yet\n", cmd)
	return ErrNotImplemented
}

// IsRouterCommand reports whether argv looks like a control command rather
// than starting a new bot session.
func IsRouterCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "status", "restart", "stop", "reload":
		return true
	default:
		return false
	}
}
