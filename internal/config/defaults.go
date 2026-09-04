// Package config defaults for runtime IPC paths (platform-specific).
package config

// DefaultSocketPath is the control endpoint when runtime.socket_path is empty.
// Unix: relative sock in $PWD; Windows: named pipe (see defaults_windows.go).

// DefaultPIDPath is where the pidfile lives when runtime.pid_path is empty.
const DefaultPIDPath = "t3b.pid"
