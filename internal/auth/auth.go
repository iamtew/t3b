// Package auth matches IRC hostmasks for owner/admin gates.
// Command dispatch is not here yet — this is the shared matcher for later PRIVMSG handlers.
package auth

import (
	"path"
	"strings"
)

// Hostmasks holds the configured owner plus admin list from t3b.conf.
type Hostmasks struct {
	Owner  string
	Admins []string
}

// New builds a Hostmasks set from config strings.
func New(owner string, admins []string) Hostmasks {
	return Hostmasks{Owner: owner, Admins: admins}
}

// IsOwner reports whether mask matches the configured owner hostmask.
func (h Hostmasks) IsOwner(mask string) bool {
	return Match(h.Owner, mask)
}

// IsAdmin reports whether mask matches any configured admin (owner is not implied).
func (h Hostmasks) IsAdmin(mask string) bool {
	for _, a := range h.Admins {
		if Match(a, mask) {
			return true
		}
	}
	return false
}

// IsOwnerOrAdmin is the usual gate for privileged bot commands.
func (h Hostmasks) IsOwnerOrAdmin(mask string) bool {
	return h.IsOwner(mask) || h.IsAdmin(mask)
}

// Match compares a pattern hostmask to a concrete nick!user@host.
// Patterns may use * wildcards (shell-style) on nick, user, or host parts,
// e.g. "*!~tew@*.example.host". Exact string match also works.
func Match(pattern, mask string) bool {
	pattern = strings.TrimSpace(pattern)
	mask = strings.TrimSpace(mask)
	if pattern == "" || mask == "" {
		return false
	}
	if strings.EqualFold(pattern, mask) {
		return true
	}

	pNick, pUser, pHost, pOK := splitMask(pattern)
	mNick, mUser, mHost, mOK := splitMask(mask)
	if !pOK || !mOK {
		// Fallback: whole-string glob when either side is not a full mask.
		ok, err := path.Match(strings.ToLower(pattern), strings.ToLower(mask))
		return err == nil && ok
	}

	return globEq(pNick, mNick) && globEq(pUser, mUser) && globEq(pHost, mHost)
}

func splitMask(mask string) (nick, user, host string, ok bool) {
	bang := strings.IndexByte(mask, '!')
	at := strings.LastIndexByte(mask, '@')
	if bang < 0 || at < 0 || at < bang {
		return "", "", "", false
	}
	return mask[:bang], mask[bang+1 : at], mask[at+1:], true
}

func globEq(pattern, value string) bool {
	ok, err := path.Match(strings.ToLower(pattern), strings.ToLower(value))
	return err == nil && ok
}
