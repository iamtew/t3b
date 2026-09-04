//go:build windows

package config

// DefaultSocketPath is a local named pipe (one-instance assumption for v1).
const DefaultSocketPath = `\\.\pipe\t3b`
