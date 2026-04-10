package shared

import (
	"strings"
	"testing"
	"time"
)

func TestRenderMemBar_Boundaries(t *testing.T) {
	tests := []struct {
		name string
		pct  float64
		want string
	}{
		{"zero", 0.0, " 0%"},
		{"fifty", 0.5, "50%"},
		{"sixty", 0.6, "60%"},
		{"eighty", 0.8, "80%"},
		{"full", 1.0, "100%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := RenderMemBar(tt.pct, 14)
			if !strings.Contains(bar, tt.want) {
				t.Errorf("RenderMemBar(%v) = %q, want substring %q", tt.pct, bar, tt.want)
			}
		})
	}
}

func TestRenderMemBar_HasBlocks(t *testing.T) {
	bar := RenderMemBar(0.5, 14)
	if !strings.Contains(bar, "█") {
		t.Error("expected filled blocks")
	}
	if !strings.Contains(bar, "░") {
		t.Error("expected empty blocks")
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		dur  time.Duration
		want string
	}{
		{2*24*time.Hour + 3*time.Hour, "2d3h"},
		{5 * time.Hour, "5h0m"},
		{1*time.Hour + 30*time.Minute, "1h30m"},
		{30 * time.Minute, "0h30m"},
	}
	for _, tt := range tests {
		got := FormatUptime(tt.dur)
		if got != tt.want {
			t.Errorf("FormatUptime(%v) = %q, want %q", tt.dur, got, tt.want)
		}
	}
}
