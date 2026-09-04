// Package resolve fetches titles / tweet / YouTube metadata for channel URLs.
package resolve

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/iamtew/t3b/internal/config"
)

// MaxIRCLine keeps PRIVMSG replies IRC-safe.
const MaxIRCLine = 350

// urlRe finds http(s) URLs conservatively (no spaces / angle brackets).
var urlRe = regexp.MustCompile(`https?://[^\s<>"']+`)

// Resolver turns one URL into a channel reply line.
type Resolver interface {
	// Match reports whether this resolver handles the URL.
	Match(u *url.URL) bool
	// Resolve returns reply text; ok=false means silent skip; err is logged by caller.
	Resolve(ctx context.Context, u *url.URL) (reply string, ok bool, err error)
}

// Engine picks the first URL in a message and runs the matching resolver.
type Engine struct {
	log       *log.Logger
	cfg       config.Resolve
	client    *http.Client
	resolvers []Resolver

	mu        sync.Mutex
	rate      map[string][]time.Time // channel -> resolution timestamps
	maxPerMin int
}

// New builds an Engine from config.
func New(logger *log.Logger, cfg config.Resolve) *Engine {
	if logger == nil {
		logger = log.Default()
	}
	e := &Engine{
		log:       logger,
		cfg:       cfg,
		client:    &http.Client{Timeout: cfg.HTTPTimeout(), CheckRedirect: limitRedirects(5)},
		rate:      make(map[string][]time.Time),
		maxPerMin: 6,
	}
	e.rebuild()
	return e
}

func limitRedirects(n int) func(*http.Request, []*http.Request) error {
	return func(_ *http.Request, via []*http.Request) error {
		if len(via) >= n {
			return http.ErrUseLastResponse
		}
		return nil
	}
}

// UpdateConfig refreshes toggles / UA / timeout after reload.
func (e *Engine) UpdateConfig(cfg config.Resolve) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg = cfg
	e.client.Timeout = cfg.HTTPTimeout()
	e.rebuildLocked()
}

func (e *Engine) rebuild() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rebuildLocked()
}

func (e *Engine) rebuildLocked() {
	e.resolvers = nil
	// Order: Twitter and YouTube before generic title so specialised handlers win.
	if e.cfg.TwitterOn() {
		e.resolvers = append(e.resolvers, &Twitter{client: e.client, ua: e.cfg.UserAgent})
	}
	// YouTube Data API only when a key is set; otherwise URLTitle handles the link.
	if e.cfg.YouTubeOn() {
		if key := strings.TrimSpace(e.cfg.YouTubeAPIKey); key != "" {
			e.resolvers = append(e.resolvers, &YouTube{client: e.client, ua: e.cfg.UserAgent, key: key})
		}
	}
	if e.cfg.URLTitlesOn() {
		e.resolvers = append(e.resolvers, &URLTitle{client: e.client, ua: e.cfg.UserAgent})
	}
}

// HandleMessage resolves the first URL in text for a channel. Empty reply = nothing to say.
func (e *Engine) HandleMessage(ctx context.Context, channel, text string) string {
	raw := FirstURL(text)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	if !e.allow(channel) {
		return ""
	}

	e.mu.Lock()
	resolvers := append([]Resolver(nil), e.resolvers...)
	e.mu.Unlock()

	for _, r := range resolvers {
		if !r.Match(u) {
			continue
		}
		reply, ok, err := r.Resolve(ctx, u)
		if err != nil {
			e.log.Printf("resolve %s: %v", raw, err)
			return ""
		}
		if !ok || reply == "" {
			return ""
		}
		return TrimIRC(reply)
	}
	return ""
}

func (e *Engine) allow(channel string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	stamps := e.rate[channel]
	kept := stamps[:0]
	for _, t := range stamps {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= e.maxPerMin {
		e.rate[channel] = kept
		return false
	}
	e.rate[channel] = append(kept, now)
	return true
}

// FirstURL returns the first http(s) URL in text, trimming trailing punctuation.
func FirstURL(text string) string {
	m := urlRe.FindString(text)
	if m == "" {
		return ""
	}
	return strings.TrimRight(m, ".,);]!?")
}

// TrimIRC truncates a reply to MaxIRCLine runes-ish (byte-safe cut).
func TrimIRC(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) <= MaxIRCLine {
		return s
	}
	return s[:MaxIRCLine-3] + "..."
}
