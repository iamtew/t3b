//go:build windows

package daemon

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/iamtew/t3b/internal/config"
)

// Listen opens a Windows named pipe.
func Listen(addr string) (net.Listener, error) {
	return winio.ListenPipe(addr, nil)
}

// Dial connects to a Windows named pipe.
func Dial(addr string) (net.Conn, error) {
	return winio.DialPipe(addr, durationPtr(3*time.Second))
}

func durationPtr(d time.Duration) *time.Duration { return &d }

// Cleanup is a no-op for named pipes.
func Cleanup(addr string) {}

func isAddrInUse(err error) bool {
	if err == nil {
		return false
	}
	// winio returns a path/pipe error when the pipe already exists.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already") || strings.Contains(msg, "in use") ||
		strings.Contains(msg, "cannot create") || strings.Contains(msg, "file exists")
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
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "T3B_DAEMON_WORKER=1")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000008 | 0x00000200, // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
	}
	if err := cmd.Start(); err != nil {
		return err
	}

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
	return fmt.Errorf("daemon child did not open control pipe at %s", addr)
}

func loadRuntimeConfig(configPath string) (*config.Config, error) {
	if configPath == "" {
		configPath = config.DefaultPath
	}
	return config.Load(configPath)
}
