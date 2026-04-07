package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/app"
	"github.com/larkly/lazytalos/internal/shared"
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

	if debug {
		if err := shared.EnableDebug(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not enable debug logging: %v\n", err)
		}
	}

	m := app.New(app.Options{
		Talosconfig:     talosconfig,
		Context:         contextFlag,
		RefreshInterval: time.Duration(refresh) * time.Second,
		PickContext:     pickContext,
		Version:         version,
		Plain:           plain,
	})

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	shared.CloseDebug()

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if fm, ok := finalModel.(app.Model); ok && fm.ShouldRestart() {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot find executable for restart: %v\n", err)
			os.Exit(1)
		}
		if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
			fmt.Fprintf(os.Stderr, "restart failed: %v\n", err)
			os.Exit(1)
		}
	}
}
