// Package bot owns the IRC session, lifecycle hooks, and handler wiring.
package bot

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iamtew/t3b/internal/auth"
	"github.com/iamtew/t3b/internal/automode"
	"github.com/iamtew/t3b/internal/commands"
	"github.com/iamtew/t3b/internal/config"
	"github.com/iamtew/t3b/internal/linklog"
	"github.com/iamtew/t3b/internal/resolve"
	"github.com/iamtew/t3b/internal/version"
	"github.com/lrstanley/girc"
)

// pageSession holds remaining .link / .more results for one nick.
type pageSession struct {
	entries []linklog.Entry
	offset  int
}

// Status is the snapshot returned by Status() / daemon status.
type Status struct {
	PID      int      `json:"pid"`
	Uptime   string   `json:"uptime"`
	Server   string   `json:"server"`
	Nick     string   `json:"nick"`
	Channels []string `json:"channels"`
	Daemon   bool     `json:"daemon"`
	Running  bool     `json:"running"`
}

// Bot owns one IRC network session plus shared lifecycle for IRC + daemon IPC.
type Bot struct {
	configPath string
	daemonMode bool
	log        *log.Logger
	started    time.Time

	mu     sync.Mutex
	cfg    *config.Config
	auth   auth.Hostmasks
	auto   *automode.Tracker
	engine *resolve.Engine
	links  *linklog.Store

	// Per-nick .more pagination for public link search.
	pageMu sync.Mutex
	pages  map[string]*pageSession

	client *girc.Client

	// Lifecycle coordination for Meat Bags / daemon control.
	rootCancel    context.CancelFunc
	sessionCancel context.CancelFunc
	wantRestart   atomic.Bool
	sessionMu     sync.Mutex // serialises runSession
}

// Options configures New.
type Options struct {
	ConfigPath string
	Daemon     bool
	Logger     *log.Logger
}

// New builds a Bot from validated config.
func New(cfg *config.Config, opts Options) *Bot {
	logger := opts.Logger
	if logger == nil {
		logger = log.New(os.Stdout, "t3b: ", log.LstdFlags)
	}
	h := auth.New(cfg.Owner, cfg.Admins)
	b := &Bot{
		cfg:        cfg,
		configPath: opts.ConfigPath,
		daemonMode: opts.Daemon,
		auth:       h,
		auto:       automode.New(h),
		engine:     resolve.New(logger, cfg.Resolve),
		log:        logger,
		started:    time.Now(),
		pages:      make(map[string]*pageSession),
	}
	b.openLinkLog(cfg)
	return b
}

// openLinkLog opens (or reopens) the JSONL beside the config file.
func (b *Bot) openLinkLog(cfg *config.Config) {
	path := b.configPath
	if path == "" {
		path = config.DefaultPath
	}
	logPath := linklog.PathFor(path, cfg.Identity.Nick, cfg.Server.Host)
	store, err := linklog.Open(logPath)
	if err != nil {
		b.log.Printf("linklog open %s: %v", logPath, err)
		return
	}
	b.mu.Lock()
	old := b.links
	b.links = store
	b.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	b.log.Printf("linklog %s", filepath.Base(logPath))
}

// Auth exposes hostmask helpers.
func (b *Bot) Auth() auth.Hostmasks {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.auth
}

// Config returns a snapshot pointer (treat as read-only).
func (b *Bot) Config() *config.Config {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cfg
}

// SetRootCancel lets main wire the process-wide cancel used by Stop().
func (b *Bot) SetRootCancel(cancel context.CancelFunc) {
	b.mu.Lock()
	b.rootCancel = cancel
	b.mu.Unlock()
}

// Run connects (and reconnects on Restart) until ctx is cancelled or Stop exits.
func (b *Bot) Run(ctx context.Context) error {
	for {
		err := b.runSession(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if b.wantRestart.Load() {
			b.wantRestart.Store(false)
			b.log.Println("restarting IRC session")
			continue
		}
		if err != nil {
			return err
		}
		return nil
	}
}

func (b *Bot) runSession(ctx context.Context) error {
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()

	b.mu.Lock()
	cfg := b.cfg
	b.mu.Unlock()

	gcfg := girc.Config{
		Server: cfg.Server.Host,
		Port:   cfg.Server.Port,
		Nick:   cfg.Identity.Nick,
		User:   cfg.Identity.User,
		Name:   cfg.Identity.Realname,
		SSL:    cfg.Server.TLS,
		// CTCP VERSION reply — stamped build id, not girc / Go runtime.
		Version: version.CTCP(),
	}
	if cfg.Server.TLS && cfg.Server.TLSSkipVerify {
		// Lab nets only — Meat Bags should keep this false on real networks.
		gcfg.TLSConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	applySASL(b.log, &gcfg, cfg.SASL, cfg.Server.TLS)

	client := girc.New(gcfg)
	b.mu.Lock()
	b.client = client
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		if b.client == client {
			b.client = nil
		}
		b.mu.Unlock()
	}()

	b.registerHandlers(client)

	sessionCtx, sessionCancel := context.WithCancel(ctx)
	b.mu.Lock()
	b.sessionCancel = sessionCancel
	b.mu.Unlock()
	defer func() {
		sessionCancel()
		b.mu.Lock()
		if b.sessionCancel != nil {
			// Clear only if still ours.
			b.sessionCancel = nil
		}
		b.mu.Unlock()
	}()

	errCh := make(chan error, 1)
	go func() {
		<-sessionCtx.Done()
		b.log.Println("session end — quitting IRC")
		_ = client.Cmd.SendRaw("QUIT :t3b shutting down")
		client.Close()
	}()

	b.log.Printf("connecting to %s (tls=%v)", cfg.Server.Address(), cfg.Server.TLS)
	go func() {
		errCh <- client.Connect()
	}()

	select {
	case err := <-errCh:
		if err != nil && ctx.Err() == nil && sessionCtx.Err() == nil && !b.wantRestart.Load() {
			return fmt.Errorf("irc connect: %w", err)
		}
		return nil
	case <-sessionCtx.Done():
		<-errCh
		return nil
	}
}

func (b *Bot) registerHandlers(client *girc.Client) {
	client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, _ girc.Event) {
		b.log.Println("connected")
		b.mu.Lock()
		channels := append([]string(nil), b.cfg.Channels...)
		b.mu.Unlock()
		for _, ch := range channels {
			b.log.Printf("joining %s", ch)
			c.Cmd.Join(ch)
		}
	})

	client.Handlers.Add(girc.JOIN, func(c *girc.Client, e girc.Event) {
		who := ""
		nick := ""
		if e.Source != nil {
			who = e.Source.String()
			nick = e.Source.Name
		}
		channel := e.Last()
		b.log.Printf("JOIN %s -> %s", who, channel)

		b.mu.Lock()
		enabled := b.cfg.Automode.AutomodeOn()
		b.mu.Unlock()
		if !enabled || e.Source == nil {
			return
		}
		// If we joined, scan for privileged users once we know we are op (MODE may lag).
		if strings.EqualFold(nick, c.GetNick()) {
			return
		}
		if !b.botIsOp(c, channel) {
			return
		}
		if b.auto.ShouldOp(channel, nick, who) {
			c.Cmd.Mode(channel, "+o", nick)
		}
	})

	client.Handlers.Add(girc.MODE, func(c *girc.Client, e girc.Event) {
		b.mu.Lock()
		enabled := b.cfg.Automode.AutomodeOn()
		b.mu.Unlock()
		if !enabled || len(e.Params) < 2 {
			return
		}
		channel := e.Params[0]
		if !girc.IsValidChannel(channel) {
			return
		}
		// When we gain +o, op privileged users already present.
		if !modeGivesUsOp(e, c.GetNick()) {
			return
		}
		b.opPrivilegedInChannel(c, channel)
	})

	client.Handlers.Add(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		target := ""
		if len(e.Params) > 0 {
			target = e.Params[0]
		}
		from := ""
		nick := ""
		if e.Source != nil {
			from = e.Source.String()
			nick = e.Source.Name
		}
		text := e.Last()
		// Own PRIVMSG is logged in sendPRIVMSG; many nets never echo it back.
		if e.Source != nil && strings.EqualFold(e.Source.Name, c.GetNick()) {
			return
		}
		b.log.Printf("PRIVMSG %s <%s> %s", target, from, text)

		if girc.IsValidChannel(target) {
			b.handleChannelPRIVMSG(c, target, nick, from, text)
			return
		}
		// DM to bot — privileged + public commands.
		if e.Source == nil {
			return
		}
		b.dispatchCommand(c, e.Source.Name, from, nick, text)
	})

	client.Handlers.Add(girc.CLOSED, func(_ *girc.Client, _ girc.Event) {
		b.log.Println("connection closed")
	})

	client.Handlers.Add(girc.ERROR, func(_ *girc.Client, e girc.Event) {
		b.log.Printf("ERROR from server: %s", e.Last())
	})
}

func (b *Bot) handleChannelPRIVMSG(c *girc.Client, channel, nick, mask, text string) {
	// Public .link / .more in-channel; never run admin/owner cmds here.
	if name, _, ok := commands.Parse(text); ok && commands.IsPublic(name) {
		b.dispatchCommand(c, channel, mask, nick, text)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), b.Config().Resolve.HTTPTimeout()+2*time.Second)
	defer cancel()
	reply := b.engine.HandleMessage(ctx, channel, text)
	if reply == "" {
		return
	}
	rawURL := resolve.FirstURL(text)
	if rawURL != "" {
		b.mu.Lock()
		store := b.links
		b.mu.Unlock()
		if store != nil {
			if _, err := store.Append(channel, nick, rawURL, reply); err != nil {
				b.log.Printf("linklog append: %v", err)
			}
		}
	}
	b.sendPRIVMSG(c, channel, reply)
}

// dispatchCommand runs commands.Dispatch and sends each reply line to target.
func (b *Bot) dispatchCommand(c *girc.Client, target, mask, nick, text string) {
	lines, err := commands.Dispatch(b.Auth(), mask, nick, text, b)
	if err != nil {
		b.log.Printf("command error: %v", err)
		b.sendPRIVMSG(c, target, "error: "+err.Error())
		return
	}
	for _, line := range lines {
		if line != "" {
			b.sendPRIVMSG(c, target, line)
		}
	}
}

// sendPRIVMSG writes a PRIVMSG and logs the same line Meat Bags see in-channel.
// Incoming handler logs everyone else; we log our own here because servers
// often do not echo our messages (the log otherwise looks like t3b is mute).
func (b *Bot) sendPRIVMSG(c *girc.Client, target, text string) {
	b.log.Printf("PRIVMSG %s <%s> %s", target, c.GetNick(), text)
	c.Cmd.Message(target, text)
}

func (b *Bot) botIsOp(c *girc.Client, channel string) bool {
	u := c.LookupUser(c.GetNick())
	if u == nil {
		return false
	}
	perms, ok := u.Perms.Lookup(channel)
	return ok && perms.Op
}

func modeGivesUsOp(e girc.Event, nick string) bool {
	// MODE #chan +o nick  — params[1] is modes, rest are nick args.
	if len(e.Params) < 3 {
		return false
	}
	modes := e.Params[1]
	args := e.Params[2:]
	adding := false
	ai := 0
	for _, r := range modes {
		switch r {
		case '+':
			adding = true
		case '-':
			adding = false
		case 'o':
			if ai < len(args) {
				if adding && strings.EqualFold(args[ai], nick) {
					return true
				}
				ai++
			}
		default:
			// Modes that take nick args: o,v,h,q,a,b,e,I — approximate: only track o here.
			if strings.ContainsRune("vhqa", r) && ai < len(args) {
				ai++
			}
		}
	}
	return false
}

func (b *Bot) opPrivilegedInChannel(c *girc.Client, channel string) {
	ch := c.LookupChannel(channel)
	if ch == nil {
		return
	}
	for _, u := range ch.Users(c) {
		if u == nil {
			continue
		}
		mask := fmt.Sprintf("%s!%s@%s", u.Nick, u.Ident, u.Host)
		if b.auto.ShouldOp(channel, u.Nick, mask) {
			c.Cmd.Mode(channel, "+o", u.Nick)
		}
	}
}

// --- commands.IRC + lifecycle ------------------------------------------------

func (b *Bot) withClient(fn func(*girc.Client) error) error {
	b.mu.Lock()
	c := b.client
	b.mu.Unlock()
	if c == nil {
		return fmt.Errorf("not connected")
	}
	return fn(c)
}

// Join implements commands.IRC.
func (b *Bot) Join(channel string) error {
	return b.withClient(func(c *girc.Client) error {
		c.Cmd.Join(channel)
		b.mu.Lock()
		b.cfg.Channels = appendUnique(b.cfg.Channels, channel)
		b.mu.Unlock()
		return nil
	})
}

// Leave implements commands.IRC.
func (b *Bot) Leave(channel string) error {
	return b.withClient(func(c *girc.Client) error {
		c.Cmd.Part(channel)
		b.mu.Lock()
		b.cfg.Channels = removeChannel(b.cfg.Channels, channel)
		b.mu.Unlock()
		return nil
	})
}

// Op implements commands.IRC.
func (b *Bot) Op(nick, channel string) error {
	return b.withClient(func(c *girc.Client) error {
		c.Cmd.Mode(channel, "+o", nick)
		return nil
	})
}

// Deop implements commands.IRC.
func (b *Bot) Deop(nick, channel string) error {
	return b.withClient(func(c *girc.Client) error {
		c.Cmd.Mode(channel, "-o", nick)
		return nil
	})
}

// Say implements commands.IRC.
func (b *Bot) Say(channel, text string) error {
	return b.withClient(func(c *girc.Client) error {
		b.sendPRIVMSG(c, channel, text)
		return nil
	})
}

// Nick implements commands.IRC.
func (b *Bot) Nick(newNick string) error {
	return b.withClient(func(c *girc.Client) error {
		c.Cmd.Nick(newNick)
		return nil
	})
}

// Stop cancels the root context so the process exits after QUIT.
func (b *Bot) Stop() error {
	b.mu.Lock()
	cancel := b.rootCancel
	b.mu.Unlock()
	if cancel != nil {
		cancel()
		return nil
	}
	return fmt.Errorf("stop: no root cancel wired")
}

// Restart reconnects IRC in-process; control socket stays up.
func (b *Bot) Restart() error {
	b.wantRestart.Store(true)
	b.mu.Lock()
	cancel := b.sessionCancel
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

// Reload re-reads TOML and applies hot vs cold rules.
func (b *Bot) Reload() error {
	path := b.configPath
	if path == "" {
		path = config.DefaultPath
	}
	neu, err := config.Load(path)
	if err != nil {
		return err
	}

	b.mu.Lock()
	old := b.cfg
	needReconnect := config.NeedsReconnect(old, neu)
	b.cfg = neu
	b.auth = auth.New(neu.Owner, neu.Admins)
	b.auto.UpdateAuth(b.auth)
	b.engine.UpdateConfig(neu.Resolve)
	client := b.client
	b.mu.Unlock()

	// Reopen link log if nick/host (or path) would change the filename.
	oldPath := linklog.PathFor(path, old.Identity.Nick, old.Server.Host)
	newPath := linklog.PathFor(path, neu.Identity.Nick, neu.Server.Host)
	if oldPath != newPath {
		b.openLinkLog(neu)
	}

	if needReconnect {
		b.log.Println("reload: cold settings changed — restarting IRC")
		return b.Restart()
	}

	// Hot: sync channel list and hostmasks (already updated).
	if client != nil {
		b.syncChannels(client, old.Channels, neu.Channels)
	}
	b.log.Println("reload: applied hot settings")
	return nil
}

func (b *Bot) syncChannels(c *girc.Client, oldCh, newCh []string) {
	oldSet := map[string]bool{}
	for _, ch := range oldCh {
		oldSet[strings.ToLower(ch)] = true
	}
	newSet := map[string]bool{}
	for _, ch := range newCh {
		newSet[strings.ToLower(ch)] = true
		if !oldSet[strings.ToLower(ch)] {
			c.Cmd.Join(ch)
		}
	}
	for _, ch := range oldCh {
		if !newSet[strings.ToLower(ch)] {
			c.Cmd.Part(ch)
		}
	}
}

// Status returns a process/IRC snapshot for Meat Bags.
func (b *Bot) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()
	nick := b.cfg.Identity.Nick
	channels := append([]string(nil), b.cfg.Channels...)
	running := b.client != nil
	if b.client != nil {
		if n := b.client.GetNick(); n != "" {
			nick = n
		}
		if list := b.client.ChannelList(); len(list) > 0 {
			channels = list
		}
	}
	return Status{
		PID:      os.Getpid(),
		Uptime:   time.Since(b.started).Truncate(time.Second).String(),
		Server:   b.cfg.Server.Address(),
		Nick:     nick,
		Channels: channels,
		Daemon:   b.daemonMode,
		Running:  running,
	}
}

// StatusText is the short IRC .status reply.
func (b *Bot) StatusText() string {
	s := b.Status()
	return fmt.Sprintf("up %s | %s as %s | chans %s | daemon=%v",
		s.Uptime, s.Server, s.Nick, strings.Join(s.Channels, ","), s.Daemon)
}

func (b *Bot) linkStore() (*linklog.Store, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.links == nil {
		return nil, fmt.Errorf("not open")
	}
	return b.links, nil
}

// LinkStats implements commands.IRC.
func (b *Bot) LinkStats() (linklog.Stats, error) {
	s, err := b.linkStore()
	if err != nil {
		return linklog.Stats{}, err
	}
	return s.Stats(), nil
}

// LinkByID implements commands.IRC.
func (b *Bot) LinkByID(id int) (linklog.Entry, bool, error) {
	s, err := b.linkStore()
	if err != nil {
		return linklog.Entry{}, false, err
	}
	e, ok := s.ByID(id)
	return e, ok, nil
}

// LinkLast implements commands.IRC.
func (b *Bot) LinkLast(n int) ([]linklog.Entry, error) {
	s, err := b.linkStore()
	if err != nil {
		return nil, err
	}
	return s.Last(n), nil
}

// LinkSearch implements commands.IRC.
func (b *Bot) LinkSearch(q string) ([]linklog.Entry, error) {
	s, err := b.linkStore()
	if err != nil {
		return nil, err
	}
	return s.Search(q), nil
}

// LinkStartPage implements commands.IRC — stores pagination and returns the first page.
func (b *Bot) LinkStartPage(nick string, entries []linklog.Entry) []string {
	lines, next, hasMore := linklog.FormatPage(entries, 0)
	key := strings.ToLower(nick)
	b.pageMu.Lock()
	if hasMore {
		b.pages[key] = &pageSession{entries: entries, offset: next}
	} else {
		delete(b.pages, key)
	}
	b.pageMu.Unlock()
	return lines
}

// LinkMore implements commands.IRC — next page for nick, or nil if none.
func (b *Bot) LinkMore(nick string) []string {
	key := strings.ToLower(nick)
	b.pageMu.Lock()
	sess := b.pages[key]
	if sess == nil {
		b.pageMu.Unlock()
		return nil
	}
	lines, next, hasMore := linklog.FormatPage(sess.entries, sess.offset)
	if hasMore {
		sess.offset = next
	} else {
		delete(b.pages, key)
	}
	b.pageMu.Unlock()
	return lines
}

func appendUnique(list []string, ch string) []string {
	for _, x := range list {
		if strings.EqualFold(x, ch) {
			return list
		}
	}
	return append(list, ch)
}

func removeChannel(list []string, ch string) []string {
	out := list[:0]
	for _, x := range list {
		if !strings.EqualFold(x, ch) {
			out = append(out, x)
		}
	}
	return out
}
