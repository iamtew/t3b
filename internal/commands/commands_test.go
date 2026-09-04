package commands

import (
	"testing"

	"github.com/iamtew/t3b/internal/auth"
)

func TestParse(t *testing.T) {
	name, args, ok := Parse(".JOIN #foo")
	if !ok || name != "join" || len(args) != 1 || args[0] != "#foo" {
		t.Fatalf("Parse JOIN: name=%q args=%v ok=%v", name, args, ok)
	}
	if _, _, ok := Parse("hello"); ok {
		t.Fatal("expected non-command")
	}
	if _, _, ok := Parse("."); ok {
		t.Fatal("expected bare dot to fail")
	}
}

func TestAllowedRoles(t *testing.T) {
	h := auth.New("owner!~o@h", []string{"admin!~a@h"})
	if !Allowed(h, "owner!~o@h", RoleOwner) {
		t.Fatal("owner should run owner cmds")
	}
	if Allowed(h, "admin!~a@h", RoleOwner) {
		t.Fatal("admin must not run owner cmds")
	}
	if !Allowed(h, "admin!~a@h", RoleAdmin) {
		t.Fatal("admin should run admin cmds")
	}
}

type stubIRC struct {
	joined, left, said           string
	stopped, restarted, reloaded bool
}

func (s *stubIRC) Join(ch string) error      { s.joined = ch; return nil }
func (s *stubIRC) Leave(ch string) error     { s.left = ch; return nil }
func (s *stubIRC) Op(_, _ string) error      { return nil }
func (s *stubIRC) Deop(_, _ string) error    { return nil }
func (s *stubIRC) Say(ch, text string) error { s.said = ch + ":" + text; return nil }
func (s *stubIRC) Nick(string) error         { return nil }
func (s *stubIRC) Stop() error               { s.stopped = true; return nil }
func (s *stubIRC) Restart() error            { s.restarted = true; return nil }
func (s *stubIRC) Reload() error             { s.reloaded = true; return nil }
func (s *stubIRC) StatusText() string        { return "ok-status" }

func TestDispatchJoinAndStop(t *testing.T) {
	h := auth.New("owner!~o@h", []string{"admin!~a@h"})
	irc := &stubIRC{}
	reply, err := Dispatch(h, "admin!~a@h", ".join #x", irc)
	if err != nil || irc.joined != "#x" || reply == "" {
		t.Fatalf("join: reply=%q err=%v joined=%q", reply, err, irc.joined)
	}
	reply, err = Dispatch(h, "admin!~a@h", ".stop", irc)
	if err != nil || reply != "permission denied" || irc.stopped {
		t.Fatalf("admin stop should deny: reply=%q stopped=%v err=%v", reply, irc.stopped, err)
	}
	reply, err = Dispatch(h, "owner!~o@h", ".stop", irc)
	if err != nil || !irc.stopped {
		t.Fatalf("owner stop: reply=%q err=%v", reply, err)
	}
}
