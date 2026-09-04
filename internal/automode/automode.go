// Package automode keeps owner/admin nicks opped when the bot holds +o.
package automode

import (
	"sync"
	"time"

	"github.com/iamtew/t3b/internal/auth"
)

// Debounce avoids MODE storms after netsplits (per nick+channel).
const Debounce = 3 * time.Second

// Tracker decides who should receive +o and rate-limits repeats.
type Tracker struct {
	auth auth.Hostmasks
	mu   sync.Mutex
	last map[string]time.Time // key: channel\x00nick
}

// New builds a Tracker from current hostmasks.
func New(h auth.Hostmasks) *Tracker {
	return &Tracker{
		auth: h,
		last: make(map[string]time.Time),
	}
}

// UpdateAuth refreshes hostmasks after config reload.
func (t *Tracker) UpdateAuth(h auth.Hostmasks) {
	t.mu.Lock()
	t.auth = h
	t.mu.Unlock()
}

// ShouldOp reports whether mask is owner/admin and debounce allows another +o.
func (t *Tracker) ShouldOp(channel, nick, mask string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.auth.IsOwnerOrAdmin(mask) {
		return false
	}
	key := channel + "\x00" + nick
	now := time.Now()
	if prev, ok := t.last[key]; ok && now.Sub(prev) < Debounce {
		return false
	}
	t.last[key] = now
	return true
}

// ShouldOpNick is for cases where we only have nick+mask (JOIN).
func (t *Tracker) ShouldOpNick(channel, nick, mask string) bool {
	return t.ShouldOp(channel, nick, mask)
}

// IsPrivileged reports owner/admin without debounce (for scans).
func (t *Tracker) IsPrivileged(mask string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.auth.IsOwnerOrAdmin(mask)
}

// NoteOp records that we just opped nick on channel (starts debounce).
func (t *Tracker) NoteOp(channel, nick string) {
	t.mu.Lock()
	t.last[channel+"\x00"+nick] = time.Now()
	t.mu.Unlock()
}
