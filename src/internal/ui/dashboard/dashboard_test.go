package dashboard

import (
	"strings"
	"testing"

	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/resources"
	"github.com/larkly/lazytalos/internal/shared"
)

func TestRenderNodeDotMatrix_Empty(t *testing.T) {
	result := RenderNodeDotMatrix(nil, nil, nil, nil, 10, 40)
	if !strings.Contains(result, "No nodes found") {
		t.Errorf("expected 'No nodes found', got %q", result)
	}
}

func TestRenderNodeDotMatrix_WithNodes(t *testing.T) {
	nodes := []cluster.NodeInfo{
		{Hostname: "cp-1", MachineType: "controlplane"},
		{Hostname: "worker-1", MachineType: "worker"},
	}
	svcs := map[string][]serviceRow{
		"cp-1":     {{ServiceID: "apid", State: "Running", Health: "OK"}},
		"worker-1": {{ServiceID: "apid", State: "Running", Health: "OK"}},
	}

	result := RenderNodeDotMatrix(nodes, svcs, nil, nil, 10, 40)
	if !strings.Contains(result, "CLUSTER NODES") {
		t.Error("expected 'CLUSTER NODES' title")
	}
	if !strings.Contains(result, "●") {
		t.Error("expected dot icons in output")
	}
	if !strings.Contains(result, "ready") {
		t.Error("expected legend in output")
	}
}

func TestRenderNodeDotMatrix_OfflineNode(t *testing.T) {
	nodes := []cluster.NodeInfo{
		{Hostname: "cp-1", MachineType: "controlplane"},
		{Hostname: "offline-1", MachineType: "worker"},
	}
	svcs := map[string][]serviceRow{
		"cp-1": {{ServiceID: "apid", State: "Running", Health: "OK"}},
		// offline-1 has no service data
	}

	result := RenderNodeDotMatrix(nodes, svcs, nil, nil, 10, 40)
	if !strings.Contains(result, "○") {
		t.Error("expected hollow circle for offline node")
	}
}

func TestRenderNodeDotMatrix_HighMemory(t *testing.T) {
	nodes := []cluster.NodeInfo{
		{Hostname: "cp-1", MachineType: "controlplane"},
	}
	svcs := map[string][]serviceRow{
		"cp-1": {{ServiceID: "apid", State: "Running", Health: "OK"}},
	}
	mem := map[string]shared.MemStats{
		"cp-1": {TotalKB: 10000, AvailableKB: 1000}, // 90% used
	}

	result := RenderNodeDotMatrix(nodes, svcs, mem, nil, 10, 40)
	// Should contain a dot (the node is shown as error due to high memory)
	if !strings.Contains(result, "●") {
		t.Error("expected dot in output")
	}
}

func TestRenderNodeDotMatrix_HighCPU(t *testing.T) {
	nodes := []cluster.NodeInfo{
		{Hostname: "cp-1", MachineType: "controlplane"},
	}
	svcs := map[string][]serviceRow{
		"cp-1": {{ServiceID: "apid", State: "Running", Health: "OK"}},
	}
	cpu := map[string]resources.CPUStats{
		"cp-1": {UsagePercent: 0.85},
	}

	result := RenderNodeDotMatrix(nodes, svcs, nil, cpu, 10, 40)
	if !strings.Contains(result, "●") {
		t.Error("expected dot in output")
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
		{"tnn3-demo-cp-1", "tnn3-demo-cp-1"},
		{"tnn3-demo-cp-1.novalocal", "tnn3-demo-cp-1"},
		{"cp-1", "cp-1"},
		{"single", "single"},
	}
	for _, tt := range tests {
		got := shared.ShortenHostname(tt.input)
		if got != tt.want {
			t.Errorf("shared.ShortenHostname(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	got := shared.Truncate("hello world", 5)
	if len([]rune(got)) > 5 {
		t.Errorf("Truncate should limit to 5 runes, got %q (runes=%d)", got, len([]rune(got)))
	}
	if got := shared.Truncate("hi", 10); got != "hi" {
		t.Errorf("Truncate should not change short strings, got %q", got)
	}
}

func TestRenderMemBar(t *testing.T) {
	bar := shared.RenderMemBar(0.5, 14)
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
	bar = shared.RenderMemBar(0.95, 14)
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
	if !strings.Contains(v, "CLUSTER NODES") {
		t.Error("expected CLUSTER NODES panel in view")
	}
	if !strings.Contains(v, "SERVICE MATRIX") {
		t.Error("expected SERVICE MATRIX panel in view")
	}
}
