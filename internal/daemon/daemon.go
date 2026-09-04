// Package daemon backgrounds the bot and routes CLI commands over a control socket.
package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/iamtew/t3b/internal/bot"
	"github.com/iamtew/t3b/internal/config"
)

// Controller is implemented by bot.Bot for IPC handlers.
type Controller interface {
	Status() bot.Status
	Stop() error
	Restart() error
	Reload() error
}

// Request is a newline-delimited JSON control message.
type Request struct {
	Cmd string `json:"cmd"`
}

// Response is the control reply.
type Response struct {
	OK     bool        `json:"ok"`
	Error  string      `json:"error,omitempty"`
	Status *bot.Status `json:"status,omitempty"`
}

// IsRouterCommand reports whether argv looks like a control command.
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

// Dispatch talks to a running instance's control endpoint.
func Dispatch(cfg *config.Config, args []string) error {
	cmd := "status"
	if len(args) > 0 {
		cmd = args[0]
	}
	addr := cfg.Runtime.SocketPathOrDefault()
	conn, err := Dial(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "t3b: no running instance at %s (%v)\n", addr, err)
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	if err := writeRequest(conn, Request{Cmd: cmd}); err != nil {
		return err
	}
	resp, err := readResponse(conn)
	if err != nil {
		return err
	}
	if !resp.OK {
		fmt.Fprintf(os.Stderr, "t3b: %s\n", resp.Error)
		return fmt.Errorf("%s", resp.Error)
	}
	if cmd == "status" && resp.Status != nil {
		s := resp.Status
		fmt.Printf("pid=%d uptime=%s server=%s nick=%s channels=%s daemon=%v running=%v\n",
			s.PID, s.Uptime, s.Server, s.Nick, strings.Join(s.Channels, ","), s.Daemon, s.Running)
	} else {
		fmt.Printf("t3b: %s ok\n", cmd)
	}
	return nil
}

func writeRequest(w io.Writer, req Request) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

func readResponse(r io.Reader) (Response, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadBytes('\n')
	if err != nil {
		return Response{}, err
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}

// Server listens for control commands for the life of the process.
type Server struct {
	addr string
	ctrl Controller
	ln   net.Listener
}

// ListenAndServe binds the control endpoint. Fails clearly if already running.
func ListenAndServe(cfg *config.Config, ctrl Controller) (*Server, error) {
	addr := cfg.Runtime.SocketPathOrDefault()
	ln, err := Listen(addr)
	if err != nil {
		if isAddrInUse(err) {
			return nil, fmt.Errorf("already running (control endpoint busy: %s)", addr)
		}
		return nil, err
	}
	s := &Server{addr: addr, ctrl: ctrl, ln: ln}
	go s.serve()
	return s, nil
}

// Addr returns the bound endpoint.
func (s *Server) Addr() string { return s.addr }

// Close stops accepting and removes Unix sockets when applicable.
func (s *Server) Close() error {
	if s == nil || s.ln == nil {
		return nil
	}
	err := s.ln.Close()
	Cleanup(s.addr)
	return err
}

func (s *Server) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	req, err := readRequest(conn)
	if err != nil {
		_ = writeResponse(conn, Response{OK: false, Error: err.Error()})
		return
	}
	resp := s.dispatch(req.Cmd)
	_ = writeResponse(conn, resp)
	// Stop after reply so the client sees ok before we exit.
	if req.Cmd == "stop" && resp.OK {
		time.AfterFunc(100*time.Millisecond, func() {
			_ = s.ctrl.Stop()
		})
	}
}

func readRequest(r io.Reader) (Request, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadBytes('\n')
	if err != nil {
		return Request{}, err
	}
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return Request{}, err
	}
	return req, nil
}

func writeResponse(w io.Writer, resp Response) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

func (s *Server) dispatch(cmd string) Response {
	switch strings.ToLower(cmd) {
	case "status":
		st := s.ctrl.Status()
		return Response{OK: true, Status: &st}
	case "restart":
		if err := s.ctrl.Restart(); err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true}
	case "reload":
		if err := s.ctrl.Reload(); err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true}
	case "stop":
		// Actual Stop() deferred in handle so the response flushes first.
		return Response{OK: true}
	default:
		return Response{OK: false, Error: "unknown command"}
	}
}

// WritePID stores the process id for Meat Bag status output.
func WritePID(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644)
}

// RemovePID deletes the pidfile if present.
func RemovePID(path string) {
	_ = os.Remove(path)
}

// MaybeDetach spawns a background worker when -daemon is set on the launcher.
// Returns (detached=true) when this process should exit after the child is ready.
func MaybeDetach(configPath string, daemonFlag bool) (detached bool, err error) {
	if !daemonFlag {
		return false, nil
	}
	if os.Getenv("T3B_DAEMON_WORKER") == "1" {
		return false, nil // we are the worker
	}
	return true, spawnDetached(configPath)
}
