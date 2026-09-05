// Command t3b is the Meat Bag entrypoint: foreground/daemon IRC bot + CLI router.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/iamtew/t3b/internal/bot"
	"github.com/iamtew/t3b/internal/config"
	"github.com/iamtew/t3b/internal/daemon"
	"github.com/iamtew/t3b/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// configWriteFlag supports -config_write and -config_write=path (IsBoolFlag).
// Space form -config_write path is handled after parse via leftover args.
type configWriteFlag struct {
	set  bool
	path string
	bare bool // true when -config_write had no =value (Set got "true")
}

func (c *configWriteFlag) String() string {
	if c.path == "" {
		return config.DefaultPath
	}
	return c.path
}

func (c *configWriteFlag) Set(s string) error {
	c.set = true
	if s == "true" {
		c.bare = true
		c.path = config.DefaultPath
		return nil
	}
	if s == "false" {
		c.set = false
		c.bare = false
		c.path = ""
		return nil
	}
	c.bare = false
	c.path = s
	return nil
}

func (c *configWriteFlag) IsBoolFlag() bool { return true }

func run(args []string) int {
	fs := flag.NewFlagSet("t3b", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultPath, "path to TOML config (default: discover *t3b.conf in $PWD)")
	var writeFlag configWriteFlag
	fs.Var(&writeFlag, "config_write", "write example config to $PWD and exit (optional path; default t3b.conf)")
	daemonMode := fs.Bool("daemon", false, "run in background")
	showVersion := fs.Bool("version", false, "print build version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Println(version.String())
		return 0
	}

	rest := fs.Args()

	// -config_write [path]: write example and leave; Meat Bags edit before connecting.
	if writeFlag.set {
		path := writeFlag.path
		if writeFlag.bare && len(rest) > 0 {
			path = rest[0]
			rest = rest[1:]
		}
		if len(rest) > 0 {
			fmt.Fprintf(os.Stderr, "t3b: unknown arguments after -config_write: %v\n", rest)
			return 2
		}
		if err := config.WriteExample(path); err != nil {
			fmt.Fprintf(os.Stderr, "t3b: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "t3b: wrote example config %q — edit it, then run t3b again\n", path)
		return 0
	}

	explicitConfig := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "config" {
			explicitConfig = true
		}
	})

	resolved := *configPath
	if !explicitConfig {
		path, err := config.Discover(".")
		if err != nil {
			if errors.Is(err, config.ErrNoConfig) {
				if werr := config.WriteExample(config.DefaultPath); werr != nil {
					fmt.Fprintf(os.Stderr, "t3b: no config found, and could not write %q: %v\n", config.DefaultPath, werr)
					return 1
				}
				fmt.Fprintf(os.Stderr, "t3b: no config found — wrote a fresh %q\n", config.DefaultPath)
				fmt.Fprintf(os.Stderr, "t3b: edit that file (server, nick, owner, channels), then run t3b again\n")
				return 1
			}
			var many *config.ErrManyConfigs
			if errors.As(err, &many) {
				fmt.Fprintf(os.Stderr, "t3b: %v\n", err)
				return 1
			}
			fmt.Fprintf(os.Stderr, "t3b: %v\n", err)
			return 1
		}
		resolved = path
	}

	// CLI router: t3b status | restart | stop | reload → talk to running instance.
	if daemon.IsRouterCommand(rest) {
		cfg, err := config.Load(resolved)
		if err != nil {
			fmt.Fprintf(os.Stderr, "t3b: %v\n", err)
			return 1
		}
		if err := daemon.Dispatch(cfg, rest); err != nil {
			return 1
		}
		return 0
	}
	if len(rest) > 0 {
		fmt.Fprintf(os.Stderr, "t3b: unknown arguments: %v\n", rest)
		return 2
	}

	detached, err := daemon.MaybeDetach(resolved, *daemonMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "t3b: %v\n", err)
		return 1
	}
	if detached {
		return 0
	}

	cfg, err := config.Load(resolved)
	if err != nil {
		fmt.Fprintf(os.Stderr, "t3b: %v\n", err)
		return 1
	}

	asDaemon := *daemonMode || os.Getenv("T3B_DAEMON_WORKER") == "1"
	b := bot.New(cfg, bot.Options{ConfigPath: resolved, Daemon: asDaemon})

	ctrl, err := daemon.ListenAndServe(cfg, b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "t3b: %v\n", err)
		return 1
	}
	defer ctrl.Close()

	pidPath := cfg.Runtime.PIDPathOrDefault(resolved)
	if err := daemon.WritePID(pidPath); err != nil {
		fmt.Fprintf(os.Stderr, "t3b: pidfile: %v\n", err)
		return 1
	}
	defer daemon.RemovePID(pidPath)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	b.SetRootCancel(stop)

	if err := b.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "t3b: %v\n", err)
		return 1
	}
	return 0
}
