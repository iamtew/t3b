// Command t3b is the Meat Bag entrypoint: foreground IRC bot, with stubs for -daemon and CLI routing.
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
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("t3b", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultPath, "path to TOML config (default: $PWD/t3b.conf)")
	daemonMode := fs.Bool("daemon", false, "run in background (not implemented yet)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	rest := fs.Args()

	// CLI router: t3b status | restart | stop | reload → talk to daemon (stub).
	if daemon.IsRouterCommand(rest) {
		if err := daemon.Dispatch(rest); err != nil {
			return 1
		}
		return 0
	}
	if len(rest) > 0 {
		fmt.Fprintf(os.Stderr, "t3b: unknown arguments: %v\n", rest)
		return 2
	}

	if *daemonMode {
		if err := daemon.Start(); err != nil {
			return 1
		}
		return 0
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "t3b: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	b := bot.New(cfg)
	if err := b.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "t3b: %v\n", err)
		return 1
	}
	return 0
}
