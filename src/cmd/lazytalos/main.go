package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

var version = "dev"

func main() {
	var (
		talosconfig string
		contextFlag string
		refresh     int
		plain       bool
		debug       bool
		showVersion bool
		pickContext bool
	)

	flag.StringVar(&talosconfig, "talosconfig", "", "path to talosconfig (default: $TALOSCONFIG env var, then ~/.talos/config)")
	flag.StringVar(&contextFlag, "context", "", "use specific context from talosconfig")
	flag.IntVar(&refresh, "refresh", 5, "auto-refresh interval in seconds")
	flag.BoolVar(&plain, "plain", false, "disable Unicode status icons")
	flag.BoolVar(&debug, "debug", false, "write debug log to ~/.cache/lazytalos/debug.log")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&pickContext, "pick-context", false, "force showing the context picker even when only one context is configured")
	flag.Parse()

	if showVersion {
		fmt.Println("lazytalos", version)
		os.Exit(0)
	}

	// Determine talosconfig path: flag > $TALOSCONFIG env > ~/.talos/config
	if talosconfig == "" {
		talosconfig = os.Getenv("TALOSCONFIG")
	}
	if talosconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error resolving home directory: %v\n", err)
			os.Exit(1)
		}
		talosconfig = filepath.Join(home, ".talos", "config")
	}

	_ = contextFlag
	_ = refresh
	_ = plain
	_ = debug
	_ = talosconfig
	_ = pickContext

	// TODO: initialize app
	fmt.Println("lazytalos", version)
}
