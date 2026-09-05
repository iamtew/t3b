package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const oldConfMissingBits = `
channels = ["#t3b"]
owner = "tew!~tew@example.host"
admins = []

[server]
host = "irc.example.net"
port = 6697
tls = true
tls_skip_verify = false

[identity]
nick = "t3b"
user = "t3b"
realname = "tew's irc bot"

[sasl]
enabled = false
mechanism = "PLAIN"
user = ""
password = ""

[runtime]
socket_path = ""
pid_path = ""

[resolve]
url_titles = true
twitter = true
youtube = true
http_timeout_sec = 8
`

func TestCheckMissingKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t3b.conf")
	if err := os.WriteFile(path, []byte(oldConfMissingBits), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Check(path)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Valid {
		t.Fatalf("expected valid conf, got %s", rep.LoadErr)
	}
	if !containsAll(rep.Missing, "resolve.reddit", "automode.enabled") {
		t.Fatalf("missing=%v want resolve.reddit and automode.enabled", rep.Missing)
	}
	if rep.OK() {
		t.Fatal("OK should be false when keys are missing")
	}
}

func TestMergeMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t3b.conf")
	if err := os.WriteFile(path, []byte(oldConfMissingBits), 0o644); err != nil {
		t.Fatal(err)
	}

	added, err := MergeMissing(path)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(added, "resolve.reddit", "automode.enabled") {
		t.Fatalf("added=%v", added)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "reddit") {
		t.Fatalf("expected reddit in file:\n%s", text)
	}
	if !strings.Contains(text, "[automode]") {
		t.Fatalf("expected [automode] in file:\n%s", text)
	}
	// Custom values preserved.
	if !strings.Contains(text, `nick = "t3b"`) || !strings.Contains(text, "irc.example.net") {
		t.Fatalf("custom values lost:\n%s", text)
	}

	rep, err := Check(path)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.OK() {
		t.Fatalf("after merge want OK, missing=%v valid=%v err=%s", rep.Missing, rep.Valid, rep.LoadErr)
	}

	// Second merge is a no-op.
	added2, err := MergeMissing(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(added2) != 0 {
		t.Fatalf("second merge added %v", added2)
	}
}

func TestCheckUnknownKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t3b.conf")
	body := oldConfMissingBits + "\n[resolve]\nreddit = true\n\n[automode]\nenabled = true\nwizardry = true\n"
	// Duplicate [resolve] is invalid TOML for BurntSushi — build a complete file instead.
	body = `
channels = ["#t3b"]
owner = "tew!~tew@example.host"
admins = []

[server]
host = "irc.example.net"
port = 6697
tls = true
tls_skip_verify = false

[identity]
nick = "t3b"
user = "t3b"
realname = "bot"

[sasl]
enabled = false
mechanism = "PLAIN"
user = ""
password = ""

[runtime]
socket_path = ""
pid_path = ""

[resolve]
url_titles = true
twitter = true
youtube = true
reddit = true
http_timeout_sec = 8

[automode]
enabled = true
wizardry = true
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := Check(path)
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(rep.Unknown, "automode.wizardry") {
		t.Fatalf("unknown=%v", rep.Unknown)
	}
	if !rep.OK() {
		t.Fatalf("unknown keys should not fail OK: missing=%v valid=%v", rep.Missing, rep.Valid)
	}
}

func containsAll(have []string, want ...string) bool {
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}
