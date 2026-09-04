# t3b — tew's IRC bot, attempt #3

IRC bot written in Go. Built with Clankers; operated by Meat Bags.

## Core features

- Justfile for build / test / run
- TLS to the IRC server (`tls_skip_verify` for lab nets only)
- SASL AUTH (PLAIN) for NickServ-style login
- Owner and admins by hostmask (`nick!~user@host.name`)
- Windows and Linux
- TOML config at `$PWD/t3b.conf`, or `-config path`
- Foreground by default; logs to the terminal; Ctrl+C / SIGTERM quits cleanly
- Daemon mode with `-daemon`
- Same binary as CLI router against a running instance (`t3b status`, `restart`, `stop`, `reload`)
- Multiple channels, one server / network

## Main features

- URL titles when an `http(s)` link is pasted in a channel
- Twitter / X: tweet text, timestamp, retweets, replies, likes, media links
- YouTube: title, channel, duration, upload date, views, likes from the public watch page (dislikes unavailable)
- Automode: while the bot is op, keep owner and admins opped
- Config and behavior commands in a **direct message** to the bot (not in channel)
- Admin: `.join #channel`, `.leave #channel`, `.op nick #channel`, `.deop nick #channel`
- Owner: `.stop`, `.restart`, `.reload`

Also available to owner/admin in DM: `.help`, `.status`, `.say #chan text`, and owner-only `.nick newnick`.

## Build / run

```bash
just build
cp t3b.conf.example t3b.conf   # edit server, nick, hostmasks, channels
just run                       # foreground
```

Useful recipes: `just test`, `just fmt`, `just tidy`, `just clean`.

Cross-build amd64 binaries from any host (including Windows) into `bin/`:

```bash
just build-windows   # bin/t3b-windows-amd64.exe
just build-linux     # bin/t3b-linux-amd64
just build-all       # both
```

### Daemon and CLI router

Only one process may own the control endpoint. Foreground mode binds it too, so `t3b status` works either way.

```bash
t3b -daemon
t3b status
t3b reload
t3b restart    # reconnects IRC in-process; control endpoint stays up
t3b stop
```

Defaults:

| Platform | Control endpoint | Pidfile |
| -------- | ---------------- | ------- |
| Unix | `$PWD/t3b.sock` | `$PWD/t3b.pid` |
| Windows | `\\.\pipe\t3b` | `$PWD/t3b.pid` |

Override under `[runtime]` in the config (`socket_path`, `pid_path`).

### Resolvers

- Channel messages only; first `http(s)` URL per line; fetch failures are logged, not spammed to IRC
- Twitter / X via [FxTwitter](https://api.fxtwitter.com)-style JSON (no OAuth)
- YouTube: public watch-page scrape for title, channel, duration, upload date, views, likes (dislikes unavailable; oEmbed is fallback)

### Privileged DM commands

Hostmask-gated. Ignored if sent in a channel. Use `.help` for the list your mask may run.

Clankers: see [AGENTS.md](AGENTS.md) for how to change this repo.
