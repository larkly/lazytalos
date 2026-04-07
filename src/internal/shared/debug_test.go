package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugfNoopWhenNotEnabled(t *testing.T) {
	// Ensure debug is disabled before the test.
	CloseDebug()

	// Debugf must not panic when debug logging is not enabled.
	Debugf("hello %s", "world")
}

func TestEnableDebugAndClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-debug.log")

	if err := EnableDebugAt(path); err != nil {
		t.Fatalf("EnableDebugAt: %v", err)
	}
	t.Cleanup(CloseDebug)

	Debugf("hello %s", "world")

	CloseDebug()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !strings.Contains(string(data), "hello world") {
		t.Errorf("expected 'hello world' in log file, got: %q", string(data))
	}
}

func TestCloseDebugIdempotent(t *testing.T) {
	// Calling CloseDebug when not enabled should not panic.
	CloseDebug()
	CloseDebug()
}
