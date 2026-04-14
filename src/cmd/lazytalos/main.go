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
	"github.com/larkly/lazytalos/internal/config"
	"github.com/larkly/lazytalos/internal/shared"
	"github.com/larkly/lazytalos/internal/talos"
)

var version = "dev"

func main() {
	var (
		talosconfig          string
		contextFlag          string
		refresh              int
		plain                bool
		debug                bool
		showVersion          bool
		pickContext          bool
		noUpdateCheck        bool
		updateCheckInterval  int
	)

	flag.StringVar(&talosconfig, "talosconfig", "", "path to talosconfig (default: $TALOSCONFIG env var, then ~/.talos/config)")
	flag.StringVar(&contextFlag, "context", "", "use specific context from talosconfig")
	flag.IntVar(&refresh, "refresh", 5, "auto-refresh interval in seconds")
	flag.BoolVar(&plain, "plain", false, "disable Unicode status icons")
	flag.BoolVar(&debug, "debug", false, "write debug log to ~/.cache/lazytalos/debug.log")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&pickContext, "pick-context", false, "force showing the context picker even when only one context is configured")
	flag.BoolVar(&noUpdateCheck, "no-update-check", false, "disable the startup self-update check")
	flag.IntVar(&updateCheckInterval, "update-check-interval", 24, "hours between update checks (0 = check every launch)")
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
	talosconfig = filepath.Clean(talosconfig)

	// Fail fast if the path doesn't point at a real file (symlinks ok as
	// long as the target resolves to a regular file).
	if err := talos.CheckTalosconfigFile(talosconfig); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	// Warn (but don't refuse) on group/world-readable talosconfig files —
	// the file holds the cluster-admin mTLS key.
	if w := talos.CheckTalosconfigPerms(talosconfig); w != "" {
		fmt.Fprintln(os.Stderr, w)
	}

	if debug {
		if err := shared.EnableDebug(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not enable debug logging: %v\n", err)
		}
	}

	// Load persisted app config, apply it, then merge CLI flag overrides.
	appCfg, err := config.Load()
	if err != nil {
		shared.Debugf("[main] config load error: %v", err)
	}
	config.ApplyAll(appCfg)

	// CLI flags override config file values where explicitly set.
	if plain {
		appCfg.General.PlainMode = true
		config.ApplyGeneral(appCfg.General)
	}
	if noUpdateCheck {
		appCfg.General.CheckForUpdates = false
	}

	opts := app.Options{
		Talosconfig:         talosconfig,
		Context:             contextFlag,
		RefreshInterval:     time.Duration(appCfg.General.RefreshInterval) * time.Second,
		PickContext:         appCfg.General.AlwaysPickContext || pickContext,
		Version:             version,
		Plain:               appCfg.General.PlainMode,
		NoUpdateCheck:       !appCfg.General.CheckForUpdates,
		UpdateCheckInterval: time.Duration(appCfg.General.UpdateCheckInterval) * time.Hour,
		AppConfig:           &appCfg,
	}
	// CLI refresh flag overrides config if not default
	if refresh != 5 {
		opts.RefreshInterval = time.Duration(refresh) * time.Second
	}
	if updateCheckInterval != 24 {
		opts.UpdateCheckInterval = time.Duration(updateCheckInterval) * time.Hour
	}

	m := app.New(opts)

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
