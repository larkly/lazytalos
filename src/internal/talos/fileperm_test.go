package talos

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCheckTalosconfigFile_Missing(t *testing.T) {
	if err := CheckTalosconfigFile(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestCheckTalosconfigFile_Directory(t *testing.T) {
	if err := CheckTalosconfigFile(t.TempDir()); err == nil {
		t.Fatal("expected error for directory")
	}
}

func TestCheckTalosconfigFile_RegularFileOK(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "talosconfig")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckTalosconfigFile(p); err != nil {
		t.Fatalf("expected no error for regular file, got %v", err)
	}
}

func TestCheckTalosconfigPerms_Private(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX perms not enforced on Windows")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "talosconfig")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if w := CheckTalosconfigPerms(p); w != "" {
		t.Fatalf("expected no warning for 0600, got %q", w)
	}
}

func TestCheckTalosconfigPerms_GroupReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX perms not enforced on Windows")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "talosconfig")
	if err := os.WriteFile(p, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	// Explicit chmod in case the umask masked the group bit above.
	if err := os.Chmod(p, 0o640); err != nil {
		t.Fatal(err)
	}
	w := CheckTalosconfigPerms(p)
	if w == "" {
		t.Fatal("expected a warning for 0640")
	}
	if !strings.Contains(w, "talosconfig") {
		t.Errorf("warning should mention talosconfig, got: %q", w)
	}
}

func TestCheckTalosconfigPerms_WorldReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX perms not enforced on Windows")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "talosconfig")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if w := CheckTalosconfigPerms(p); w == "" {
		t.Fatal("expected a warning for 0644")
	}
}

func TestCheckTalosconfigPerms_MissingFileNoWarning(t *testing.T) {
	// Missing file is CheckTalosconfigFile's job to flag; the perm check
	// should stay quiet rather than double-warning.
	if w := CheckTalosconfigPerms(filepath.Join(t.TempDir(), "nope")); w != "" {
		t.Fatalf("expected empty warning for missing file, got %q", w)
	}
}
