package shared

import "strings"

// ShortenHostname extracts the meaningful part of a FQDN hostname.
// e.g. "tnn3-demo-cp-1.novalocal" -> "cp-1"
func ShortenHostname(hostname string) string {
	// Strip domain suffix
	if idx := strings.Index(hostname, "."); idx > 0 {
		hostname = hostname[:idx]
	}
	parts := strings.Split(hostname, "-")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "-")
	}
	return hostname
}

// Truncate shortens a string to maxLen, appending an ellipsis if truncated.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "\u2026"
}
