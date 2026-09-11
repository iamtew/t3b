package karma

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPathFor(t *testing.T) {
	got := PathFor(filepath.Join("cfg", "t3b.conf"), "bot", "irc.example")
	want := filepath.Join("cfg", "karma-bot-irc.example.db")
	if got != want {
		t.Fatalf("PathFor: got %q want %q", got, want)
	}
}

func TestParseBumpPlain(t *testing.T) {
	b, ok := ParseBump("foobar++")
	if !ok || b.Phrase != "foobar" || b.Delta != 1 || b.Mode != ModeSilent || b.Reject != "" {
		t.Fatalf("foobar++: %+v ok=%v", b, ok)
	}
	b, ok = ParseBump("foo bar++")
	if !ok || b.Phrase != "foo bar" || b.Delta != 1 {
		t.Fatalf("foo bar++: %+v", b)
	}
	b, ok = ParseBump("foo bar-- baz")
	if !ok || b.Phrase != "foo bar" || b.Delta != -1 || b.Mode != ModeSilent {
		t.Fatalf("foo bar-- baz: %+v", b)
	}
	b, ok = ParseBump("food++")
	if !ok || b.Phrase != "food" || b.Delta != 1 || b.Mode != ModeSilent {
		t.Fatalf("food++ must be plain not dice: %+v", b)
	}
	if _, ok := ParseBump("++"); ok {
		t.Fatal("empty phrase")
	}
	if _, ok := ParseBump("hello world"); ok {
		t.Fatal("no operator")
	}
}

func TestParseBumpDice(t *testing.T) {
	b, ok := ParseBump("foobar+d")
	if !ok || b.Phrase != "foobar" || b.Mode != ModeDice || b.Reject != "" {
		t.Fatalf("+d: %+v ok=%v", b, ok)
	}
	if b.Result < 1 || b.Result > 6 || b.Delta != b.Result {
		t.Fatalf("+d roll: result=%d delta=%d", b.Result, b.Delta)
	}
	b, ok = ParseBump("foobar-d")
	if !ok || b.Mode != ModeDice || b.Delta != -b.Result {
		t.Fatalf("-d: %+v", b)
	}
	b, ok = ParseBump("x+d trailing")
	if !ok || b.Phrase != "x" || b.Mode != ModeDice {
		t.Fatalf("+d trailing: %+v", b)
	}
}

func TestParseBumpRandom(t *testing.T) {
	b, ok := ParseBump("foobar+1..6")
	if !ok || b.Phrase != "foobar" || b.Mode != ModeRandom || b.Reject != "" {
		t.Fatalf("+N..M: %+v ok=%v", b, ok)
	}
	if b.Result < 1 || b.Result > 6 || b.Delta != b.Result {
		t.Fatalf("+N..M roll: %+v", b)
	}
	b, ok = ParseBump("foo+2..6")
	if !ok || b.Phrase != "foo" || b.Mode != ModeRandom {
		t.Fatalf("foo+2..6: %+v ok=%v", b, ok)
	}
	b, ok = ParseBump("x-2..2")
	if !ok || b.Result != 2 || b.Delta != -2 {
		t.Fatalf("-2..2: %+v", b)
	}
	b, ok = ParseBump("nope+1..24")
	if !ok || b.Reject == "" || b.Delta != 0 {
		t.Fatalf("reject hi: %+v", b)
	}
	if !strings.Contains(b.Reject, "hakuna yer tata's") {
		t.Fatalf("reject msg: %q", b.Reject)
	}
	b, ok = ParseBump("nope+24..1")
	if !ok || b.Reject == "" {
		t.Fatalf("reject either bound: %+v", b)
	}
	// ++ must not be eaten as a broken range
	b, ok = ParseBump("food++")
	if !ok || b.Mode != ModeSilent || b.Phrase != "food" {
		t.Fatalf("food++ vs range: %+v", b)
	}
}

func TestAdjustReply(t *testing.T) {
	if got := AdjustReply(ModeSilent, 1, 0, 1); got != "" {
		t.Fatalf("silent: %q", got)
	}
	got := AdjustReply(ModeDice, 4, 10, 14)
	want := "karma adjusted with dice roll 4, new karma: 10 -> 14"
	if got != want {
		t.Fatalf("dice: %q", got)
	}
	got = AdjustReply(ModeRandom, 7, 1, -6)
	want = "karma adjusted with random 7, new karma: 1 -> -6"
	if got != want {
		t.Fatalf("random: %q", got)
	}
}

func TestStoreAddGet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "karma.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	score, found, err := s.Get("gone")
	if err != nil || found || score != 0 {
		t.Fatalf("missing: score=%d found=%v err=%v", score, found, err)
	}
	from, to, err := s.Add("foo bar", 3)
	if err != nil || from != 0 || to != 3 {
		t.Fatalf("add: from=%d to=%d err=%v", from, to, err)
	}
	from, to, err = s.Add("foo bar", -1)
	if err != nil || from != 3 || to != 2 {
		t.Fatalf("add2: from=%d to=%d err=%v", from, to, err)
	}
	score, found, err = s.Get("foo bar")
	if err != nil || !found || score != 2 {
		t.Fatalf("get: score=%d found=%v err=%v", score, found, err)
	}
}
