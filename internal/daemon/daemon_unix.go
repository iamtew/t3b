//go:build !windows

package daemon

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/iamtew/t3b/internal/config"
)

// Listen opens a Unix domain socket (removes a stale sock file first).
func Listen(addr string) (net.Listener, error) {
	_ = os.Remove(addr)
	return net.Listen("unix", addr)
}

// Dial connects to a Unix domain socket.
func Dial(addr string) (net.Conn, error) {
	return net.DialTimeout("unix", addr, 3*time.Second)
}

// Cleanup removes the socket file after Close.
func Cleanup(addr string) {
	_ = os.Remove(addr)
}

func isAddrInUse(err error) bool {
	if err == nil {
		return false
	}
	if op, ok := err.(*net.OpError); ok {
		if sys, ok := op.Err.(*os.SyscallError); ok {
			return sys.Err == syscall.EADDRINUSE
		}
	}
	return false
}

func spawnDetached(configPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{}
	if configPath != "" && configPath != config.DefaultPath {
		args = append(args, "-config", configPath)
	}
	// Worker inherits T3B_DAEMON_WORKER and runs without -daemon (already backgrounded).
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "T3B_DAEMON_WORKER=1")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}

	// Wait until control socket accepts, using config defaults / file.
	cfg, err := loadRuntimeConfig(configPath)
	if err != nil {
		return fmt.Errorf("daemon child started but config load failed: %w", err)
	}
	addr := cfg.Runtime.SocketPathOrDefault()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := Dial(addr)
		if err == nil {
			_ = conn.Close()
			fmt.Fprintf(os.Stderr, "t3b: daemon started (pid %d)\n", cmd.Process.Pid)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon child did not open control socket at %s", addr)
}

func loadRuntimeConfig(configPath string) (*config.Config, error) {
	if configPath == "" {
		configPath = config.DefaultPath
	}
	return config.Load(configPath)
}
