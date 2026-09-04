// Package commands parses and dispatches privileged '.' PRIVMSG commands.
package commands

import (
	"fmt"
	"strings"

	"github.com/iamtew/t3b/internal/auth"
)

// Role is who may run a command.
type Role int

const (
	RoleAdmin Role = iota // owner or admin
	RoleOwner             // owner only
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
	case "join", "leave", "op", "deop", "status", "say", "help":
		return RoleAdmin, true
	case "stop", "restart", "reload", "nick":
		return RoleOwner, true
	default:
		return 0, false
	}
}

// Allowed reports whether masks may run a command with the given role.
func Allowed(h auth.Hostmasks, mask string, role Role) bool {
	switch role {
	case RoleOwner:
		return h.IsOwner(mask)
	default:
		return h.IsOwnerOrAdmin(mask)
	}
}

// Dispatch runs a parsed DM command. reply is sent back to the caller when non-empty.
func Dispatch(h auth.Hostmasks, mask string, text string, irc IRC) (reply string, err error) {
	name, args, ok := Parse(text)
	if !ok {
		return "", nil
	}
	role, known := RequiredRole(name)
	if !known {
		return "unknown command — try .help", nil
	}
	if !Allowed(h, mask, role) {
		return "permission denied", nil
	}

	switch name {
	case "help":
		return helpText(h, mask), nil
	case "status":
		return irc.StatusText(), nil
	case "join":
		if len(args) < 1 {
			return "usage: .join #channel", nil
		}
		if err := irc.Join(args[0]); err != nil {
			return "", err
		}
		return "joining " + args[0], nil
	case "leave":
		if len(args) < 1 {
			return "usage: .leave #channel", nil
		}
		if err := irc.Leave(args[0]); err != nil {
			return "", err
		}
		return "leaving " + args[0], nil
	case "op":
		if len(args) < 2 {
			return "usage: .op nick #channel", nil
		}
		if err := irc.Op(args[0], args[1]); err != nil {
			return "", err
		}
		return fmt.Sprintf("op %s on %s", args[0], args[1]), nil
	case "deop":
		if len(args) < 2 {
			return "usage: .deop nick #channel", nil
		}
		if err := irc.Deop(args[0], args[1]); err != nil {
			return "", err
		}
		return fmt.Sprintf("deop %s on %s", args[0], args[1]), nil
	case "say":
		if len(args) < 2 {
			return "usage: .say #channel text", nil
		}
		msg := strings.Join(args[1:], " ")
		if err := irc.Say(args[0], msg); err != nil {
			return "", err
		}
		return "ok", nil
	case "nick":
		if len(args) < 1 {
			return "usage: .nick newnick", nil
		}
		if err := irc.Nick(args[0]); err != nil {
			return "", err
		}
		return "nick -> " + args[0], nil
	case "stop":
		if err := irc.Stop(); err != nil {
			return "", err
		}
		return "stopping", nil
	case "restart":
		if err := irc.Restart(); err != nil {
			return "", err
		}
		return "restarting IRC", nil
	case "reload":
		if err := irc.Reload(); err != nil {
			return err.Error(), nil
		}
		return "reloaded", nil
	default:
		return "unknown command — try .help", nil
	}
}

func helpText(h auth.Hostmasks, mask string) string {
	parts := []string{".help", ".status", ".join", ".leave", ".op", ".deop", ".say"}
	if h.IsOwner(mask) {
		parts = append(parts, ".nick", ".stop", ".restart", ".reload")
	}
	return "commands: " + strings.Join(parts, " ")
}
