package linklog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathFor(t *testing.T) {
	p := PathFor(filepath.Join("conf", "t3b.conf"), "t3b", "irc.example.net")
	want := filepath.Join("conf", "links-t3b-irc.example.net.log")
	if p != want {
		t.Fatalf("PathFor: got %q want %q", p, want)
	}
}

func TestAppendSearchLastStatsByID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "links-bot-host.log")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	e1, err := s.Append("#chan", "alice", "https://youtube.com/watch?v=1", "YouTube: one | ch")
	if err != nil {
		t.Fatal(err)
	}
	if e1.ID != 1 || e1.Domain != "youtube.com" {
		t.Fatalf("e1: %+v", e1)
	}
	e2, err := s.Append("#chan", "bob", "https://x.com/u/status/2", "@u: tweet")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Append("#other", "carol", "https://reddit.com/r/x/comments/abc", "Reddit: post")
	if err != nil {
		t.Fatal(err)
	}

	st := s.Stats()
	if st.Total != 3 || st.Domains != 3 {
		t.Fatalf("stats: %+v", st)
	}

	got, ok := s.ByID(2)
	if !ok || got.URL != e2.URL {
		t.Fatalf("ByID: ok=%v got=%+v", ok, got)
	}
	if _, ok := s.ByID(99); ok {
		t.Fatal("expected miss")
	}

	last := s.Last(2)
	if len(last) != 2 || last[0].ID != 3 || last[1].ID != 2 {
		t.Fatalf("Last: %+v", last)
	}

	hits := s.Search("youtube")
	if len(hits) != 1 || hits[0].ID != 1 {
		t.Fatalf("Search youtube: %+v", hits)
	}
	hits = s.Search("tweet")
	if len(hits) != 1 || hits[0].ID != 2 {
		t.Fatalf("Search title: %+v", hits)
	}
	hits = s.Search("HTTPS://X.COM")
	if len(hits) != 1 {
		t.Fatalf("Search URL case: %+v", hits)
	}

	// Re-open and ensure ids continue.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	e4, err := s2.Append("#chan", "dave", "https://example.com/", "Title: ex")
	if err != nil {
		t.Fatal(err)
	}
	if e4.ID != 4 {
		t.Fatalf("next id after reopen: %d", e4.ID)
	}
	if s2.Stats().Total != 4 {
		t.Fatalf("reopen total %d", s2.Stats().Total)
	}
}

func TestFormatPage(t *testing.T) {
	entries := []Entry{
		{ID: 1, Channel: "#a", User: "u", URL: "https://a", Title: "t1"},
		{ID: 2, Channel: "#a", User: "u", URL: "https://b", Title: "t2"},
		{ID: 3, Channel: "#a", User: "u", URL: "https://c", Title: "t3"},
		{ID: 4, Channel: "#a", User: "u", URL: "https://d", Title: "t4"},
	}
	lines, next, more := FormatPage(entries, 0)
	if !more || next != 3 || len(lines) != 4 {
		t.Fatalf("page1: lines=%d next=%d more=%v %v", len(lines), next, more, lines)
	}
	if lines[3] != "Showing 1-3 of 4. Send .more or .m for next." {
		t.Fatalf("footer: %q", lines[3])
	}
	lines, next, more = FormatPage(entries, next)
	if more || next != 4 || len(lines) != 2 {
		t.Fatalf("page2: lines=%d next=%d more=%v %v", len(lines), next, more, lines)
	}
	if lines[1] != "Showing 4-4 of 4." {
		t.Fatalf("last footer: %q", lines[1])
	}
}

func TestOpenCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "links-n-h.log")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
