package shared

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	debugEnabled bool
	debugLogger  *log.Logger
	debugMu      sync.Mutex
	debugFile    *os.File
)

// EnableDebug opens a debug log file under the user cache directory.
func EnableDebug() error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("determine user cache dir: %w", err)
	}
	dir := filepath.Join(cacheDir, "lazytalos")
	// Restrict to owner-only: the debug log may capture machine config
	// YAML, error messages wrapping credentials, and other sensitive
	// data that shouldn't be readable by other local users.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, "debug.log")
	fmt.Fprintf(os.Stderr, "Debug log: %s\n", path)
	return enableDebugAt(path)
}

// enableDebugAt opens (or creates) a log file at the given path and enables debug logging.
// It is used by EnableDebug and by tests via EnableDebugAt.
func enableDebugAt(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	debugMu.Lock()
	defer debugMu.Unlock()
	debugFile = f
	debugLogger = log.New(f, "", 0)
	debugEnabled = true
	debugLogger.Printf("=== debug started at %s ===", time.Now().Format(time.RFC3339))
	return nil
}

// EnableDebugAt opens a debug log file at the given path. Intended for tests.
func EnableDebugAt(path string) error {
	return enableDebugAt(path)
}

// DebugEnabled returns true if debug logging is active.
func DebugEnabled() bool {
	debugMu.Lock()
	defer debugMu.Unlock()
	return debugEnabled
}

// CloseDebug flushes and closes the debug log file and disables further logging.
func CloseDebug() {
	debugMu.Lock()
	defer debugMu.Unlock()
	if debugFile != nil {
		_ = debugFile.Close()
		debugFile = nil
	}
	debugLogger = nil
	debugEnabled = false
}

// Debugf writes a timestamped message to the debug log.
func Debugf(format string, args ...any) {
	debugMu.Lock()
	defer debugMu.Unlock()
	if debugLogger == nil {
		return
	}
	debugLogger.Printf("%s %s", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}
