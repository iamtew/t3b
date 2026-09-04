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
- Owner commands include: .stop, .restart, .reload, 