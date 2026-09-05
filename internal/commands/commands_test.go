package commands

import (
	"testing"

	"github.com/iamtew/t3b/internal/auth"
	"github.com/iamtew/t3b/internal/linklog"
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
	if !Allowed(h, "rando!~r@h", RolePublic) {
		t.Fatal("anyone should run public cmds")
	}
}

func TestIsPublic(t *testing.T) {
	if !IsPublic("link") || !IsPublic("l") || !IsPublic("more") || !IsPublic("m") {
		t.Fatal("expected public aliases")
	}
	if IsPublic("join") || IsPublic("stop") || IsPublic("nope") {
		t.Fatal("expected non-public")
	}
}

type stubIRC struct {
	joined, left, said           string
	stopped, restarted, reloaded bool

	stats   linklog.Stats
	byID    map[int]linklog.Entry
	last    []linklog.Entry
	search  []linklog.Entry
	pages   map[string]*pageStub
	started []linklog.Entry
}

type pageStub struct {
	entries []linklog.Entry
	offset  int
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

func (s *stubIRC) LinkStats() (linklog.Stats, error) { return s.stats, nil }
func (s *stubIRC) LinkByID(id int) (linklog.Entry, bool, error) {
	e, ok := s.byID[id]
	return e, ok, nil
}
func (s *stubIRC) LinkLast(n int) ([]linklog.Entry, error) {
	if n > len(s.last) {
		n = len(s.last)
	}
	return s.last[:n], nil
}
func (s *stubIRC) LinkSearch(string) ([]linklog.Entry, error) { return s.search, nil }
func (s *stubIRC) LinkStartPage(nick string, entries []linklog.Entry) []string {
	s.started = entries
	lines, next, more := linklog.FormatPage(entries, 0)
	if s.pages == nil {
		s.pages = map[string]*pageStub{}
	}
	key := nick
	if more {
		s.pages[key] = &pageStub{entries: entries, offset: next}
	} else {
		delete(s.pages, key)
	}
	return lines
}
func (s *stubIRC) LinkMore(nick string) []string {
	sess := s.pages[nick]
	if sess == nil {
		return nil
	}
	lines, next, more := linklog.FormatPage(sess.entries, sess.offset)
	if more {
		sess.offset = next
	} else {
		delete(s.pages, nick)
	}
	return lines
}

func TestDispatchJoinAndStop(t *testing.T) {
	h := auth.New("owner!~o@h", []string{"admin!~a@h"})
	irc := &stubIRC{}
	lines, err := Dispatch(h, "admin!~a@h", "admin", ".join #x", irc)
	if err != nil || irc.joined != "#x" || len(lines) == 0 {
		t.Fatalf("join: lines=%v err=%v joined=%q", lines, err, irc.joined)
	}
	lines, err = Dispatch(h, "admin!~a@h", "admin", ".stop", irc)
	if err != nil || lines[0] != "permission denied" || irc.stopped {
		t.Fatalf("admin stop should deny: lines=%v stopped=%v err=%v", lines, irc.stopped, err)
	}
	lines, err = Dispatch(h, "owner!~o@h", "owner", ".stop", irc)
	if err != nil || !irc.stopped {
		t.Fatalf("owner stop: lines=%v err=%v", lines, err)
	}
}

func TestDispatchLinkAliasesAndPagination(t *testing.T) {
	h := auth.New("owner!~o@h", nil)
	entries := []linklog.Entry{
		{ID: 1, Channel: "#a", User: "u", URL: "https://a.example/1", Title: "one"},
		{ID: 2, Channel: "#a", User: "u", URL: "https://a.example/2", Title: "two"},
		{ID: 3, Channel: "#a", User: "u", URL: "https://a.example/3", Title: "three"},
		{ID: 4, Channel: "#a", User: "u", URL: "https://a.example/4", Title: "four"},
	}
	irc := &stubIRC{
		stats:  linklog.Stats{Total: 4, Domains: 1},
		byID:   map[int]linklog.Entry{2: entries[1]},
		last:   []linklog.Entry{entries[3], entries[2], entries[1], entries[0]},
		search: entries,
	}

	lines, err := Dispatch(h, "rando!~r@x", "rando", ".l", irc)
	if err != nil || len(lines) != 1 || lines[0][:6] != "links:" {
		t.Fatalf("bare .l: %v err=%v", lines, err)
	}

	lines, err = Dispatch(h, "rando!~r@x", "rando", ".link 2", irc)
	if err != nil || len(lines) != 1 || lines[0][:2] != "#2" {
		t.Fatalf("by id: %v", lines)
	}

	lines, err = Dispatch(h, "rando!~r@x", "rando", ".l s example", irc)
	if err != nil || len(lines) != 4 {
		t.Fatalf("search page: %v", lines)
	}
	if lines[3] != "Showing 1-3 of 4. Send .more or .m for next." {
		t.Fatalf("footer: %q", lines[3])
	}

	lines, err = Dispatch(h, "rando!~r@x", "rando", ".m", irc)
	if err != nil || len(lines) != 2 {
		t.Fatalf("more: %v", lines)
	}
	lines, err = Dispatch(h, "rando!~r@x", "rando", ".more", irc)
	if err != nil || lines[0] != "no more results" {
		t.Fatalf("exhausted: %v", lines)
	}

	lines, err = Dispatch(h, "rando!~r@x", "rando", ".l l", irc)
	if err != nil || len(irc.started) != 3 {
		t.Fatalf(".l l should last 3: started=%d lines=%v", len(irc.started), lines)
	}

	lines, err = Dispatch(h, "rando!~r@x", "rando", ".link last 2", irc)
	if err != nil || len(irc.started) != 2 {
		t.Fatalf(".link last 2: started=%d lines=%v", len(irc.started), lines)
	}
}
