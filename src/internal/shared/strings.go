package shared

import "strings"

// ShortenHostname strips domain suffixes from a FQDN hostname but preserves
// the full short name. e.g. "talos-cp-1.novalocal" -> "talos-cp-1".
// IPv6 addresses and bare IPs are returned as-is.
func ShortenHostname(hostname string) string {
	// IPv6 or IPv4 — return as-is
	if strings.Contains(hostname, ":") || isIPv4(hostname) {
		return hostname
	}
	// Strip domain suffix
	if idx := strings.Index(hostname, "."); idx > 0 {
		hostname = hostname[:idx]
	}
	return hostname
}

func isIPv4(s string) bool {
	dots := 0
	for _, c := range s {
		if c == '.' {
			dots++
		} else if c < '0' || c > '9' {
			return false
		}
	}
	return dots == 3
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
