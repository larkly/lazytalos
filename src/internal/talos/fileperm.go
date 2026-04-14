package talos

import (
	"fmt"
	"os"
	"runtime"
)

// CheckTalosconfigFile verifies that path exists and refers to a regular
// file. It is intentionally strict: talosconfig embeds the cluster-admin
// mTLS client cert and key, so pointing it at a device, directory, or
// arbitrary symlink target is almost always a mistake or an attack.
func CheckTalosconfigFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat talosconfig %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("talosconfig %s is not a regular file (mode=%s)", path, info.Mode())
	}
	return nil
}

// CheckTalosconfigPerms returns a human-readable warning if the talosconfig
// at path is readable by group or other, or an empty string if the file is
// private enough. Callers should print the warning to stderr and continue —
// this is a soft check, not a hard refusal, so users with unusual home
// layouts can still operate.
//
// On Windows the check is a no-op because POSIX mode bits don't carry the
// same meaning; Windows relies on ACLs which Go's os.FileInfo doesn't
// expose portably.
func CheckTalosconfigPerms(path string) string {
	if runtime.GOOS == "windows" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	mode := info.Mode().Perm()
	// 0o077 catches any group/other read/write/execute bits.
	if mode&0o077 == 0 {
		return ""
	}
	return fmt.Sprintf(
		"warning: talosconfig %s has permissive mode %#o; it contains the cluster mTLS client key — consider chmod 600",
		path, mode,
	)
}
