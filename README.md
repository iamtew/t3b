# t3b - tew's irc bot, attempt #3

IRC bot written in Go.

Work in progress... With the Clanker, obviously.

## Core Features (in no particular order):
- Uses Justfile for build management
- SSL TLS support
- SASL AUTH for NickServ and such
- Bot owner and admins based on hostmask like: `nick!~user@host.name`
- Runs on both Windows and Linux
- Uses TOML config file located at `$PWD/t3b.conf` or specified with `-config` flag
- Foreground mode, default mode, spits out its activities in the terminal where it's started from. Ctrl+C will shutdown bot gracefully.
- Daemon mode, runs in background when started with `-daemon` flag
- `t3b` binary can be used as command router on daemonized instance. for example if one is running `t3b -daemon` I can in another window type `t3b status` or `t3b restart` and it will look into the running one, not start a new one.
- can join multiple channels, set in config file
- can only join one server/network, set in config file

## Main features (in no particular order):
- URL resolution, display Title of a web page when a URL is patesed in IRC channel
- Twitter / X.Com resolution: Display Tweet plus timestamp, retweets, comments, likes, direct links to media if any present.
- YouTube resolution: Display Video title, Channel name, Duration, Upload date, Likes/Dislikes
- Automode: Ensure Owner and admins are always opped in channel bot is opped.
- Config + behavior commands available to owner and admin in `PRIVMSG` with bot.
- Admin commmands include: .join #channel, .leave #channel, .op user #channel, .deop user #channel
- Owner commands include: .stop, .restart, .reload

## Build / run

```bash
just build
cp t3b.conf.example t3b.conf   # edit hostmasks, server, channels
just run                       # foreground (Ctrl+C quits)
```

### Daemon + CLI router

Only one instance may own the control endpoint (Unix socket or Windows named pipe). Foreground mode also binds it, so `t3b status` works against either.

```bash
t3b -daemon
t3b status
t3b reload
t3b restart   # reconnects IRC in-process; control endpoint stays up
t3b stop
```

Defaults: Unix `$PWD/t3b.sock` + `$PWD/t3b.pid`; Windows `\\.\pipe\t3b` + `$PWD/t3b.pid`. Override under `[runtime]` in the config.

### Resolvers

- Channel PRIVMSG only; first `http(s)` URL per message; failures are logged, not spammed to the channel.
- Twitter/X via [FxTwitter](https://api.fxtwitter.com)-style JSON (no OAuth).
- YouTube: set `[youtube] api_key` for Data API v3 (title, channel, duration, upload date, likes). Without a key, oEmbed returns title + channel only. **Dislikes are unavailable from Google** — t3b reports likes only.

### Privileged DM commands

Owner/admin commands are accepted only in a direct message to the bot nick (not in channel). Extra behavior commands: `.status`, `.say #chan text`, `.nick newnick` (owner), `.help`.

End of file.
