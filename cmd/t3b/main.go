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

func run(args []string) int {
	fs := flag.NewFlagSet("t3b", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultPath, "path to TOML config (default: $PWD/t3b.conf)")
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

	// CLI router: t3b status | restart | stop | reload → talk to running instance.
	if daemon.IsRouterCommand(rest) {
		cfg, err := config.Load(*configPath)
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

	detached, err := daemon.MaybeDetach(*configPath, *daemonMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "t3b: %v\n", err)
		return 1
	}
	if detached {
		return 0
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "t3b: %v\n", err)
		return 1
	}

	asDaemon := *daemonMode || os.Getenv("T3B_DAEMON_WORKER") == "1"
	b := bot.New(cfg, bot.Options{ConfigPath: *configPath, Daemon: asDaemon})

	ctrl, err := daemon.ListenAndServe(cfg, b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "t3b: %v\n", err)
		return 1
	}
	defer ctrl.Close()

	pidPath := cfg.Runtime.PIDPathOrDefault()
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
