package config

import (
	"os"
	"path/filepath"
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
}

func TestValidateRejectsEmpty(t *testing.T) {
	var cfg Config
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
