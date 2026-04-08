package update

import (
	"context"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v1.1.0", "v1.0.0", true},
		{"v1.0.0", "v1.1.0", false},
		{"v1.0.0", "v1.0.0", false},
		{"v1.1.0", "dev", false},
		{"v1.1.0", "", false},
		{"v1.0.1", "v1.0.0", true},
		{"v2.0.0", "v1.9.9", true},
		{"v1.0.0", "v2.0.0", false},
	}

	for _, tc := range cases {
		got := IsNewer(tc.latest, tc.current)
		if got != tc.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

func TestCheckLatest_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	r, err := CheckLatest(ctx)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if r != nil {
		t.Errorf("expected nil release for cancelled context, got %v", r)
	}
}
