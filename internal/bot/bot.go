// Package bot runs the foreground IRC client: connect, join, log, shut down cleanly.
package bot

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/iamtew/t3b/internal/auth"
	"github.com/iamtew/t3b/internal/config"
	"github.com/lrstanley/girc"
)

// Bot owns the IRC session for one network.
type Bot struct {
	cfg  *config.Config
	auth auth.Hostmasks
	log  *log.Logger
}

// New builds a Bot from validated config. Hostmasks are kept ready for later commands.
func New(cfg *config.Config) *Bot {
	return &Bot{
		cfg:  cfg,
		auth: auth.New(cfg.Owner, cfg.Admins),
		log:  log.New(os.Stdout, "t3b: ", log.LstdFlags),
	}
}

// Auth exposes hostmask helpers for future PRIVMSG command gates.
func (b *Bot) Auth() auth.Hostmasks {
	return b.auth
}

// Run connects, joins channels, logs traffic, and returns when ctx is cancelled
// or the IRC session ends.
func (b *Bot) Run(ctx context.Context) error {
	gcfg := girc.Config{
		Server: b.cfg.Server.Host,
		Port:   b.cfg.Server.Port,
		Nick:   b.cfg.Identity.Nick,
		User:   b.cfg.Identity.User,
		Name:   b.cfg.Identity.Realname,
		SSL:    b.cfg.Server.TLS,
	}

	if b.cfg.Server.TLS && b.cfg.Server.TLSSkipVerify {
		// Lab nets only — Meat Bags should keep this false on real networks.
		gcfg.TLSConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}

	// SASL is reserved in config; live auth is a later pass (README: SASL AUTH).
	applySASLStub(b.log, &gcfg, b.cfg.SASL)

	client := girc.New(gcfg)
	b.registerHandlers(client)

	// When Meat Bags hit Ctrl+C, cancel ctx and tear down the IRC session.
	errCh := make(chan error, 1)
	go func() {
		<-ctx.Done()
		b.log.Println("shutdown signal — quitting IRC")
		_ = client.Cmd.SendRaw("QUIT :t3b shutting down")
		client.Close()
	}()

	b.log.Printf("connecting to %s (tls=%v)", b.cfg.Server.Address(), b.cfg.Server.TLS)
	go func() {
		errCh <- client.Connect()
	}()

	select {
	case err := <-errCh:
		if err != nil && ctx.Err() == nil {
			return fmt.Errorf("irc connect: %w", err)
		}
		// Context cancel or clean Close() — treat as graceful exit.
		return nil
	case <-ctx.Done():
		err := <-errCh
		if err != nil {
			b.log.Printf("connect ended after shutdown: %v", err)
		}
		return nil
	}
}

func (b *Bot) registerHandlers(client *girc.Client) {
	client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, _ girc.Event) {
		b.log.Println("connected")
		for _, ch := range b.cfg.Channels {
			b.log.Printf("joining %s", ch)
			c.Cmd.Join(ch)
		}
	})

	client.Handlers.Add(girc.JOIN, func(_ *girc.Client, e girc.Event) {
		who := ""
		if e.Source != nil {
			who = e.Source.String()
		}
		b.log.Printf("JOIN %s -> %s", who, e.Last())
	})

	client.Handlers.Add(girc.PRIVMSG, func(_ *girc.Client, e girc.Event) {
		target := ""
		if len(e.Params) > 0 {
			target = e.Params[0]
		}
		from := ""
		if e.Source != nil {
			from = e.Source.String()
		}
		b.log.Printf("PRIVMSG %s <%s> %s", target, from, e.Last())
	})

	client.Handlers.Add(girc.CLOSED, func(_ *girc.Client, _ girc.Event) {
		b.log.Println("connection closed")
	})

	client.Handlers.Add(girc.ERROR, func(_ *girc.Client, e girc.Event) {
		b.log.Printf("ERROR from server: %s", e.Last())
	})
}

// applySASLStub documents the future SASL hook without enabling girc SASL yet.
func applySASLStub(logger *log.Logger, _ *girc.Config, sasl config.SASL) {
	if !sasl.Enabled {
		return
	}
	mech := strings.TrimSpace(sasl.Mechanism)
	if mech == "" {
		mech = "PLAIN"
	}
	logger.Printf("SASL enabled in config (mechanism=%s) but not wired yet — skipping auth", mech)
}
