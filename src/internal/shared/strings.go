package shared

import "strings"

// ShortenHostname extracts the meaningful part of a FQDN hostname.
// e.g. "tnn3-demo-cp-1.novalocal" -> "cp-1"
// IPv6 addresses and IPs are returned with a short suffix.
func ShortenHostname(hostname string) string {
	// IPv6 address — show last segment
	if strings.Contains(hostname, ":") {
		parts := strings.Split(hostname, ":")
		return "…" + parts[len(parts)-1]
	}
	// IPv4 address — show last two octets
	if isIPv4(hostname) {
		parts := strings.Split(hostname, ".")
		if len(parts) == 4 {
			return parts[2] + "." + parts[3]
		}
	}
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
