// Package config loads and validates t3b's TOML config for Meat Bags.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const DefaultPath = "t3b.conf"

// Config is the full on-disk shape of t3b.conf.
type Config struct {
	Server   Server   `toml:"server"`
	Identity Identity `toml:"identity"`
	SASL     SASL     `toml:"sasl"`
	Channels []string `toml:"channels"`
	Owner    string   `toml:"owner"`
	Admins   []string `toml:"admins"`
	Runtime  Runtime  `toml:"runtime"`
	Resolve  Resolve  `toml:"resolve"`
	YouTube  YouTube  `toml:"youtube"`
	Automode Automode `toml:"automode"`
}

// Server is the single IRC network t3b connects to.
type Server struct {
	Host          string `toml:"host"`
	Port          int    `toml:"port"`
	TLS           bool   `toml:"tls"`
	TLSSkipVerify bool   `toml:"tls_skip_verify"`
}

// Identity is the nick/user/realname sent on connect.
type Identity struct {
	Nick     string `toml:"nick"`
	User     string `toml:"user"`
	Realname string `toml:"realname"`
}

// SASL holds NickServ-style credentials wired into girc when enabled.
type SASL struct {
	Enabled   bool   `toml:"enabled"`
	Mechanism string `toml:"mechanism"`
	User      string `toml:"user"`
	Password  string `toml:"password"`
}

// Runtime holds daemon IPC paths (socket/pipe + pidfile).
type Runtime struct {
	SocketPath string `toml:"socket_path"`
	PIDPath    string `toml:"pid_path"`
}

// Resolve toggles channel URL scanners and HTTP client knobs.
type Resolve struct {
	URLTitles      *bool  `toml:"url_titles"`
	Twitter        *bool  `toml:"twitter"`
	YouTube        *bool  `toml:"youtube"`
	UserAgent      string `toml:"user_agent"`
	HTTPTimeoutSec int    `toml:"http_timeout_sec"`
}

// YouTube holds the optional Data API v3 key.
type YouTube struct {
	APIKey string `toml:"api_key"`
}

// Automode keeps owner/admins opped when the bot is op.
// Enabled defaults to true when the key is omitted.
type Automode struct {
	Enabled *bool `toml:"enabled"`
}

// Load reads path (or DefaultPath) and validates required fields.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if _, err := toml.Decode(string(raw), &cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %q: %w", path, err)
	}
	return &cfg, nil
}

// applyDefaults fills empty optional fields so Meat Bags can omit them.
func (c *Config) applyDefaults() {
	if c.Resolve.UserAgent == "" {
		c.Resolve.UserAgent = "t3b/0.1 (+https://github.com/iamtew/t3b)"
	}
	if c.Resolve.HTTPTimeoutSec <= 0 {
		c.Resolve.HTTPTimeoutSec = 8
	}
	if c.Resolve.URLTitles == nil {
		c.Resolve.URLTitles = boolPtr(true)
	}
	if c.Resolve.Twitter == nil {
		c.Resolve.Twitter = boolPtr(true)
	}
	if c.Resolve.YouTube == nil {
		c.Resolve.YouTube = boolPtr(true)
	}
	if c.Automode.Enabled == nil {
		c.Automode.Enabled = boolPtr(true)
	}
	if c.SASL.Enabled {
		if strings.TrimSpace(c.SASL.Mechanism) == "" {
			c.SASL.Mechanism = "PLAIN"
		}
	}
}

func boolPtr(v bool) *bool { return &v }

// URLTitlesOn reports whether generic URL title resolution is enabled.
func (r Resolve) URLTitlesOn() bool { return r.URLTitles == nil || *r.URLTitles }

// TwitterOn reports whether Twitter/X resolution is enabled.
func (r Resolve) TwitterOn() bool { return r.Twitter == nil || *r.Twitter }

// YouTubeOn reports whether YouTube resolution is enabled.
func (r Resolve) YouTubeOn() bool { return r.YouTube == nil || *r.YouTube }

// AutomodeOn reports whether automode is enabled (default true).
func (a Automode) AutomodeOn() bool { return a.Enabled == nil || *a.Enabled }

// Validate fails fast with messages Meat Bags can act on.
func (c *Config) Validate() error {
	var missing []string

	if strings.TrimSpace(c.Server.Host) == "" {
		missing = append(missing, "server.host")
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		missing = append(missing, "server.port (1-65535)")
	}
	if strings.TrimSpace(c.Identity.Nick) == "" {
		missing = append(missing, "identity.nick")
	}
	if strings.TrimSpace(c.Identity.User) == "" {
		missing = append(missing, "identity.user")
	}
	if strings.TrimSpace(c.Identity.Realname) == "" {
		missing = append(missing, "identity.realname")
	}
	if len(c.Channels) == 0 {
		missing = append(missing, "channels (at least one)")
	}
	for i, ch := range c.Channels {
		if strings.TrimSpace(ch) == "" {
			missing = append(missing, fmt.Sprintf("channels[%d]", i))
		}
	}
	if strings.TrimSpace(c.Owner) == "" {
		missing = append(missing, "owner")
	}
	if c.SASL.Enabled {
		if strings.TrimSpace(c.SASL.User) == "" {
			missing = append(missing, "sasl.user (required when sasl.enabled)")
		}
		if strings.TrimSpace(c.SASL.Password) == "" {
			missing = append(missing, "sasl.password (required when sasl.enabled)")
		}
		mech := strings.ToUpper(strings.TrimSpace(c.SASL.Mechanism))
		if mech == "" {
			missing = append(missing, "sasl.mechanism (required when sasl.enabled)")
		} else if mech != "PLAIN" {
			return fmt.Errorf("unsupported sasl.mechanism %q (only PLAIN is supported)", c.SASL.Mechanism)
		}
	}
	if c.Resolve.HTTPTimeoutSec < 1 || c.Resolve.HTTPTimeoutSec > 120 {
		missing = append(missing, "resolve.http_timeout_sec (1-120)")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing or invalid: %s", strings.Join(missing, ", "))
	}
	return nil
}

// Address returns host:port for the IRC dialer.
func (s Server) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// HTTPTimeout returns the resolver HTTP client timeout.
func (r Resolve) HTTPTimeout() time.Duration {
	sec := r.HTTPTimeoutSec
	if sec <= 0 {
		sec = 8
	}
	return time.Duration(sec) * time.Second
}

// NeedsReconnect reports whether cold settings changed (server/identity/SASL/TLS).
func NeedsReconnect(old, neu *Config) bool {
	if old == nil || neu == nil {
		return true
	}
	return old.Server != neu.Server ||
		old.Identity != neu.Identity ||
		old.SASL != neu.SASL
}

// SocketPathOrDefault returns the control endpoint path.
func (r Runtime) SocketPathOrDefault() string {
	if strings.TrimSpace(r.SocketPath) != "" {
		return r.SocketPath
	}
	return DefaultSocketPath
}

// PIDPathOrDefault returns the pidfile path.
func (r Runtime) PIDPathOrDefault() string {
	if strings.TrimSpace(r.PIDPath) != "" {
		return r.PIDPath
	}
	return DefaultPIDPath
}
