# t3b — tew's IRC bot, attempt #3

IRC bot written in Go. Built with Clankers; operated by Meat Bags.

## Core features

- Justfile for build / test / run
- TLS to the IRC server (`tls_skip_verify` for lab nets only)
- SASL AUTH (PLAIN) for NickServ-style login
- Owner and admins by hostmask (`nick!~user@host.name`)
- Windows and Linux
- TOML config: auto-picks the sole `*t3b.conf` in `$PWD`, or `-config path`; `-config_write` drops an example
- Foreground by default; logs to the terminal; Ctrl+C / SIGTERM quits cleanly
- Daemon mode with `-daemon`
- Same binary as CLI router against a running instance (`t3b status`, `restart`, `stop`, `reload`)
- Multiple channels, one server / network

## Main features

- URL titles when an `http(s)` link is pasted in a channel
- Twitter / X: tweet text, timestamp, retweets, replies, likes, media links
- YouTube: Data API v3 for title, channel, duration, upload date, views, likes when `youtube_api_key` is set (plain page title if the key is omitted)
- Automode: while the bot is op, keep owner and admins opped
- Config and behavior commands in a **direct message** to the bot (not in channel)
- Admin: `.join #channel`, `.leave #channel`, `.op nick #channel`, `.deop nick #channel`
- Owner: `.stop`, `.restart`, `.reload`

Also available to owner/admin in DM: `.help`, `.status`, `.say #chan text`, and owner-only `.nick newnick`.

## Build / run

```bash
just build
t3b -config_write              # writes t3b.conf in $PWD (or: -config_write mybot.conf)
# edit server, nick, hostmasks, channels — then:
just run                       # foreground
```

On first start with no `*t3b.conf` in `$PWD`, t3b writes a fresh `t3b.conf` and exits so you can fill it in. If several files end with `t3b.conf` (e.g. `bot.t3b.conf` and `other_t3b.conf`), it refuses to guess — use `-config` or clean up. Names like `botname_t3b.conf` / `bot.t3b.conf` count.

`just build` stamps the binary version from git: short commit hash, or `tag-shorthash` when HEAD is exactly on a tag (append `-dirty` if the tree is dirty). Check with `t3b -version`. `just run` / plain `go build` fall back to the embedded VCS hash when present.

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

- Channel messages only; first `http(s)` URL anywhere in the line; fetch failures are logged, not spammed to IRC
- Twitter / X via [FxTwitter](https://api.fxtwitter.com)-style JSON (no OAuth)
- YouTube: [Data API v3](https://developers.google.com/youtube/v3) `videos.list` when `[resolve] youtube_api_key` is set (title, channel, duration, upload date, views, likes; 1 quota unit). No key → generic URL title like any other link. Do not commit the key.

### Privileged DM commands

Hostmask-gated. Ignored if sent in a channel. Use `.help` for the list your mask may run.

Clankers: see [AGENTS.md](AGENTS.md) for how to change this repo.
