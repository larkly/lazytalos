package shared

import "testing"

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"Running", "●"},
		{"OK", "●"},
		{"Stopped", "○"},
		{"Failed", "✘"},
		{"Degraded", "▲"},
		{"", "?"},
		{"Unknown", "?"},
	}

	PlainMode = false
	for _, tt := range tests {
		got := StatusIcon(tt.status)
		if got != tt.want {
			t.Errorf("StatusIcon(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestStatusIconPlainMode(t *testing.T) {
	PlainMode = true
	defer func() { PlainMode = false }()

	statuses := []string{"Running", "OK", "Stopped", "Failed", "Degraded", "", "Unknown"}
	for _, s := range statuses {
		got := StatusIcon(s)
		if got != "" {
			t.Errorf("StatusIcon(%q) with PlainMode = %q, want empty string", s, got)
		}
	}
}
