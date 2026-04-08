package dashboard

import (
	"strings"
	"testing"

	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/shared"
)

func TestRenderNodeHealth_Empty(t *testing.T) {
	result := RenderNodeHealth(nil, nil, nil, 10, 12)
	if !strings.Contains(result, "No nodes found") {
		t.Errorf("expected 'No nodes found', got %q", result)
	}
}

func TestRenderNodeHealth_WithNodes(t *testing.T) {
	nodes := []cluster.NodeInfo{
		{Hostname: "cp-1", MachineType: "controlplane"},
		{Hostname: "worker-1", MachineType: "worker"},
	}
	mem := map[string]memStats{
		"cp-1":     {TotalKB: 8000000, AvailableKB: 5000000},
		"worker-1": {TotalKB: 16000000, AvailableKB: 6000000},
	}

	result := RenderNodeHealth(nodes, nil, mem, 10, 12)
	if !strings.Contains(result, "cp-1") {
		t.Error("expected cp-1 in output")
	}
	if !strings.Contains(result, "worker-1") {
		t.Error("expected worker-1 in output")
	}
	// Should contain memory bar characters
	if !strings.Contains(result, "█") {
		t.Error("expected memory bar blocks in output")
	}
}

func TestRenderServiceMatrix_CPvsWorker(t *testing.T) {
	nodes := []cluster.NodeInfo{
		{Hostname: "test-cp-1", MachineType: "controlplane"},
		{Hostname: "test-w-1", MachineType: "worker"},
	}
	servicesByNode := map[string][]serviceRow{
		"test-cp-1": {
			{ServiceID: "etcd", State: "Running", Health: "OK"},
			{ServiceID: "trustd", State: "Running", Health: "OK"},
			{ServiceID: "apid", State: "Running", Health: "OK"},
		},
		"test-w-1": {
			{ServiceID: "apid", State: "Running", Health: "OK"},
		},
	}

	result := RenderServiceMatrix(nodes, servicesByNode, 20)

	if !strings.Contains(result, "etcd") {
		t.Error("expected etcd in service matrix")
	}
	// The "·" should appear for worker on etcd and trustd rows
	if !strings.Contains(result, "·") {
		t.Error("expected '·' for services not expected on worker")
	}
}

func TestTickMsg_TriggersRefresh(t *testing.T) {
	m := New(nil, 0)
	m2, cmd := m.Update(shared.TickMsg{})
	_ = m2
	if cmd == nil {
		t.Error("expected TickMsg to produce fetch commands")
	}
}

func TestShortenHostname(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"tnn3-demo-cp-1", "cp-1"},
		{"cp-1", "cp-1"},
		{"single", "single"},
	}
	for _, tt := range tests {
		got := shortenHostname(tt.input)
		if got != tt.want {
			t.Errorf("shortenHostname(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	got := truncate("hello world", 5)
	if len([]rune(got)) > 5 {
		t.Errorf("truncate should limit to 5 runes, got %q (runes=%d)", got, len([]rune(got)))
	}
	if got := truncate("hi", 10); got != "hi" {
		t.Errorf("truncate should not change short strings, got %q", got)
	}
}

func TestRenderMemBar(t *testing.T) {
	bar := renderMemBar(0.5, 14)
	if !strings.Contains(bar, "50%") {
		t.Errorf("expected '50%%' in bar, got %q", bar)
	}
	if !strings.Contains(bar, "█") {
		t.Error("expected filled blocks in bar")
	}
	if !strings.Contains(bar, "░") {
		t.Error("expected empty blocks in bar")
	}

	// High usage should still work
	bar = renderMemBar(0.95, 14)
	if !strings.Contains(bar, "95%") {
		t.Errorf("expected '95%%' in bar, got %q", bar)
	}
}

func TestDashboardView_Empty(t *testing.T) {
	m := New(nil, 0)
	m.width = 120
	m.height = 40
	m.loading = false

	v := m.View()
	if !strings.Contains(v, "CLUSTER STATUS") {
		t.Error("expected CLUSTER STATUS panel in view")
	}
	if !strings.Contains(v, "NODE HEALTH") {
		t.Error("expected NODE HEALTH panel in view")
	}
	if !strings.Contains(v, "SERVICE MATRIX") {
		t.Error("expected SERVICE MATRIX panel in view")
	}
}
