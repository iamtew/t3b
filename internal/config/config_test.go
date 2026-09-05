package config

import (
	"errors"
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

func TestLoadYouTubeAPIKey(t *testing.T) {
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
youtube_api_key = "  test-key  "
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Resolve.YouTubeAPIKey != "test-key" {
		t.Fatalf("key=%q", cfg.Resolve.YouTubeAPIKey)
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

func TestDiscover(t *testing.T) {
	dir := t.TempDir()

	if _, err := Discover(dir); !errors.Is(err, ErrNoConfig) {
		t.Fatalf("empty dir: got %v, want ErrNoConfig", err)
	}

	one := filepath.Join(dir, "foo_t3b.conf")
	if err := os.WriteFile(one, []byte("# hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(dir)
	if err != nil {
		t.Fatalf("one match: %v", err)
	}
	if got != one {
		t.Fatalf("path = %q, want %q", got, one)
	}

	two := filepath.Join(dir, "bar.t3b.conf")
	if err := os.WriteFile(two, []byte("# hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Discover(dir)
	var many *ErrManyConfigs
	if !errors.As(err, &many) {
		t.Fatalf("two matches: got %v, want ErrManyConfigs", err)
	}
	if len(many.Names) != 2 {
		t.Fatalf("Names = %v", many.Names)
	}
}

func TestWriteExample(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t3b.conf")

	if err := WriteExample(path); err != nil {
		t.Fatalf("WriteExample: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || !strings.Contains(string(raw), "[server]") {
		t.Fatalf("unexpected example body (%d bytes)", len(raw))
	}

	if err := WriteExample(path); !errors.Is(err, ErrExampleExists) {
		t.Fatalf("second write: got %v, want ErrExampleExists", err)
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

func TestDefaultPIDPathFor(t *testing.T) {
	tests := []struct {
		config string
		want   string
	}{
		{"", DefaultPIDPath},
		{".", DefaultPIDPath},
		{DefaultPath, DefaultPIDPath},
		{"./t3b.conf", DefaultPIDPath},
		{filepath.Join("dir", "t3b.conf"), DefaultPIDPath},
		{"bot.t3b.conf", "bot.t3b.pid"},
		{"foo_t3b.conf", "foo_t3b.pid"},
		{filepath.Join("elsewhere", "bot.t3b.conf"), "bot.t3b.pid"},
		{"custom.conf", "custom.pid"},
	}
	for _, tt := range tests {
		got := DefaultPIDPathFor(tt.config)
		if got != tt.want {
			t.Errorf("DefaultPIDPathFor(%q) = %q, want %q", tt.config, got, tt.want)
		}
	}
}

func TestPIDPathOrDefault(t *testing.T) {
	explicit := Runtime{PIDPath: "custom.pid"}
	if got := explicit.PIDPathOrDefault("bot.t3b.conf"); got != "custom.pid" {
		t.Fatalf("explicit pid_path: got %q, want custom.pid", got)
	}
	empty := Runtime{}
	if got := empty.PIDPathOrDefault("bot.t3b.conf"); got != "bot.t3b.pid" {
		t.Fatalf("derived: got %q, want bot.t3b.pid", got)
	}
}
