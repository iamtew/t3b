//go:build !windows

package config

// DefaultSocketPath is a Unix domain socket in the working directory.
const DefaultSocketPath = "t3b.sock"
