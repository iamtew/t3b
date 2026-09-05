// Package config defaults for runtime IPC paths (platform-specific).
package config

import (
	"path/filepath"
	"strings"
)

// DefaultSocketPath is the control endpoint when runtime.socket_path is empty.
// Unix: relative sock in $PWD; Windows: named pipe (see defaults_windows.go).

// DefaultPIDPath is where the pidfile lives when runtime.pid_path is empty
// and the config file is the standard DefaultPath (t3b.conf).
const DefaultPIDPath = "t3b.pid"

// DefaultPIDPathFor returns the default pidfile name for a config path.
// Standard t3b.conf → t3b.pid; non-standard names use the config basename
// stem (e.g. bot.t3b.conf → bot.t3b.pid, foo_t3b.conf → foo_t3b.pid).
func DefaultPIDPathFor(configPath string) string {
	base := filepath.Base(configPath)
	if base == "" || base == "." || base == DefaultPath {
		return DefaultPIDPath
	}
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if stem == "" {
		return DefaultPIDPath
	}
	return stem + ".pid"
}
