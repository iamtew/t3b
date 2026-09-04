package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAndValidate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t3b.conf")
	body := `
channels = ["#chan"]
owner = "tew!~tew@host"

[server]
host = "irc.example.net"
port = 6697
tls = true

[identity]
nick = "t3b"
user = "t3b"
realname = "bot"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Address() != "irc.example.net:6697" {
		t.Fatalf("Address = %q", cfg.Server.Address())
	}
	if !cfg.Resolve.URLTitlesOn() || !cfg.Automode.AutomodeOn() {
		t.Fatal("expected resolve/automode defaults on")
	}
}

func TestValidateRejectsEmpty(t *testing.T) {
	var cfg Config
	cfg.applyDefaults()
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSASLRequiresPLAIN(t *testing.T) {
	cfg := validBase()
	cfg.SASL.Enabled = true
	cfg.SASL.User = "u"
	cfg.SASL.Password = "p"
	cfg.SASL.Mechanism = "EXTERNAL"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "PLAIN") {
		t.Fatalf("expected PLAIN-only error, got %v", err)
	}

	cfg.SASL.Mechanism = "PLAIN"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("PLAIN should validate: %v", err)
	}
}

func TestResolveTogglesOff(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t3b.conf")
	body := `
channels = ["#chan"]
owner = "tew!~tew@host"

[server]
host = "irc.example.net"
port = 6697
tls = true

[identity]
nick = "t3b"
user = "t3b"
realname = "bot"

[resolve]
url_titles = false
twitter = false
youtube = false

[automode]
enabled = false
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Resolve.URLTitlesOn() || cfg.Resolve.TwitterOn() || cfg.Resolve.YouTubeOn() {
		t.Fatal("expected resolve toggles off")
	}
	if cfg.Automode.AutomodeOn() {
		t.Fatal("expected automode off")
	}
}

func validBase() Config {
	cfg := Config{
		Server:   Server{Host: "irc.example.net", Port: 6697, TLS: true},
		Identity: Identity{Nick: "t3b", User: "t3b", Realname: "bot"},
		Channels: []string{"#chan"},
		Owner:    "tew!~tew@host",
	}
	cfg.applyDefaults()
	return cfg
}
