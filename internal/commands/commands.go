// Package commands parses and dispatches '.' PRIVMSG commands.
package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/iamtew/t3b/internal/auth"
	"github.com/iamtew/t3b/internal/linklog"
)

// Role is who may run a command.
type Role int

const (
	RolePublic Role = iota // anyone
	RoleAdmin              // owner or admin
	RoleOwner              // owner only
)

// IRC is the subset of IRC ops the dispatcher needs (implemented by bot).
type IRC interface {
	Join(channel string) error
	Leave(channel string) error
	Op(nick, channel string) error
	Deop(nick, channel string) error
	Say(channel, text string) error
	Nick(newNick string) error
	Stop() error
	Restart() error
	Reload() error
	StatusText() string

	// Link log (public .link / .more).
	LinkStats() (linklog.Stats, error)
	LinkByID(id int) (linklog.Entry, bool, error)
	LinkLast(n int) ([]linklog.Entry, error)
	LinkSearch(q string) ([]linklog.Entry, error)
	LinkStartPage(nick string, entries []linklog.Entry) []string
	LinkMore(nick string) []string
}

// Parse splits ".cmd args..." — name is lowercased. ok=false if not a command.
func Parse(text string) (name string, args []string, ok bool) {
	text = strings.TrimSpace(text)
	if text == "" || text[0] != '.' {
		return "", nil, false
	}
	fields := strings.Fields(text[1:])
	if len(fields) == 0 {
		return "", nil, false
	}
	name = strings.ToLower(fields[0])
	return name, fields[1:], true
}

// RequiredRole returns the minimum role for a known command, or false if unknown.
func RequiredRole(name string) (Role, bool) {
	switch name {
	case "link", "l", "more", "m":
		return RolePublic, true
	case "join", "leave", "op", "deop", "status", "say", "help":
		return RoleAdmin, true
	case "stop", "restart", "reload", "nick":
		return RoleOwner, true
	default:
		return 0, false
	}
}

// IsPublic reports whether name is a public (no-ACL) command.
func IsPublic(name string) bool {
	role, ok := RequiredRole(name)
	return ok && role == RolePublic
}

// Allowed reports whether masks may run a command with the given role.
func Allowed(h auth.Hostmasks, mask string, role Role) bool {
	switch role {
	case RolePublic:
		return true
	case RoleOwner:
		return h.IsOwner(mask)
	default:
		return h.IsOwnerOrAdmin(mask)
	}
}

// Dispatch runs a parsed command. lines are sent back as separate PRIVMSGs when non-empty.
// nick is the caller's nickname (for .more pagination); mask is nick!user@host for ACL.
func Dispatch(h auth.Hostmasks, mask, nick, text string, irc IRC) (lines []string, err error) {
	name, args, ok := Parse(text)
	if !ok {
		return nil, nil
	}
	role, known := RequiredRole(name)
	if !known {
		return []string{"unknown command — try .help"}, nil
	}
	if !Allowed(h, mask, role) {
		return []string{"permission denied"}, nil
	}

	switch name {
	case "help":
		return []string{helpText(h, mask)}, nil
	case "status":
		return []string{irc.StatusText()}, nil
	case "join":
		if len(args) < 1 {
			return []string{"usage: .join #channel"}, nil
		}
		if err := irc.Join(args[0]); err != nil {
			return nil, err
		}
		return []string{"joining " + args[0]}, nil
	case "leave":
		if len(args) < 1 {
			return []string{"usage: .leave #channel"}, nil
		}
		if err := irc.Leave(args[0]); err != nil {
			return nil, err
		}
		return []string{"leaving " + args[0]}, nil
	case "op":
		if len(args) < 2 {
			return []string{"usage: .op nick #channel"}, nil
		}
		if err := irc.Op(args[0], args[1]); err != nil {
			return nil, err
		}
		return []string{fmt.Sprintf("op %s on %s", args[0], args[1])}, nil
	case "deop":
		if len(args) < 2 {
			return []string{"usage: .deop nick #channel"}, nil
		}
		if err := irc.Deop(args[0], args[1]); err != nil {
			return nil, err
		}
		return []string{fmt.Sprintf("deop %s on %s", args[0], args[1])}, nil
	case "say":
		if len(args) < 2 {
			return []string{"usage: .say #channel text"}, nil
		}
		msg := strings.Join(args[1:], " ")
		if err := irc.Say(args[0], msg); err != nil {
			return nil, err
		}
		return []string{"ok"}, nil
	case "nick":
		if len(args) < 1 {
			return []string{"usage: .nick newnick"}, nil
		}
		if err := irc.Nick(args[0]); err != nil {
			return nil, err
		}
		return []string{"nick -> " + args[0]}, nil
	case "stop":
		if err := irc.Stop(); err != nil {
			return nil, err
		}
		return []string{"stopping"}, nil
	case "restart":
		if err := irc.Restart(); err != nil {
			return nil, err
		}
		return []string{"restarting IRC"}, nil
	case "reload":
		if err := irc.Reload(); err != nil {
			return []string{err.Error()}, nil
		}
		return []string{"reloaded"}, nil
	case "link", "l":
		return dispatchLink(nick, args, irc)
	case "more", "m":
		out := irc.LinkMore(nick)
		if len(out) == 0 {
			return []string{"no more results"}, nil
		}
		return out, nil
	default:
		return []string{"unknown command — try .help"}, nil
	}
}

func dispatchLink(nick string, args []string, irc IRC) ([]string, error) {
	if len(args) == 0 {
		st, err := irc.LinkStats()
		if err != nil {
			return []string{"link log unavailable: " + err.Error()}, nil
		}
		return []string{fmt.Sprintf(
			"links: %d logged, %d unique domains. subcommands: search|s <q>, last|l [n], <id>. pagination: .more|.m",
			st.Total, st.Domains)}, nil
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "search", "s":
		if len(args) < 2 {
			return []string{"usage: .link search <query>"}, nil
		}
		q := strings.Join(args[1:], " ")
		entries, err := irc.LinkSearch(q)
		if err != nil {
			return []string{"link search failed: " + err.Error()}, nil
		}
		if len(entries) == 0 {
			return []string{"no results"}, nil
		}
		return irc.LinkStartPage(nick, entries), nil
	case "last", "l":
		n := linklog.PageSize
		if len(args) >= 2 {
			v, err := strconv.Atoi(args[1])
			if err != nil || v < 1 {
				return []string{"usage: .link last [n]"}, nil
			}
			n = v
		}
		entries, err := irc.LinkLast(n)
		if err != nil {
			return []string{"link last failed: " + err.Error()}, nil
		}
		if len(entries) == 0 {
			return []string{"no links logged yet"}, nil
		}
		return irc.LinkStartPage(nick, entries), nil
	default:
		id, err := strconv.Atoi(args[0])
		if err != nil || id < 1 {
			return []string{"usage: .link | .link search <q> | .link last [n] | .link <id>"}, nil
		}
		e, ok, err := irc.LinkByID(id)
		if err != nil {
			return []string{"link lookup failed: " + err.Error()}, nil
		}
		if !ok {
			return []string{fmt.Sprintf("no link with id %d", id)}, nil
		}
		return []string{linklog.FormatEntry(e)}, nil
	}
}

func helpText(h auth.Hostmasks, mask string) string {
	parts := []string{".help", ".status", ".join", ".leave", ".op", ".deop", ".say", ".link", ".more"}
	if h.IsOwner(mask) {
		parts = append(parts, ".nick", ".stop", ".restart", ".reload")
	}
	return "commands: " + strings.Join(parts, " ")
}
