// Package config loads and validates t3b's TOML config for Meat Bags.
package config

import (
	"fmt"
	"os"
	"strings"

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

// SASL holds credentials for later NickServ-style auth (not wired yet).
type SASL struct {
	Enabled   bool   `toml:"enabled"`
	Mechanism string `toml:"mechanism"`
	User      string `toml:"user"`
	Password  string `toml:"password"`
}

// Runtime reserves daemon IPC paths for a later pass.
type Runtime struct {
	SocketPath string `toml:"socket_path"`
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

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %q: %w", path, err)
	}
	return &cfg, nil
}

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
		if strings.TrimSpace(c.SASL.Mechanism) == "" {
			missing = append(missing, "sasl.mechanism (required when sasl.enabled)")
		}
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
