package dashboard

import (
	"strings"
	"testing"

	"github.com/larkly/lazytalos/internal/cluster"
	"github.com/larkly/lazytalos/internal/shared"
)

func TestRenderNodeTable_Empty(t *testing.T) {
	result := RenderNodeTable(nil, nil, nil, 80, 10)
	if !strings.Contains(result, "No nodes found") {
		t.Errorf("expected 'No nodes found', got %q", result)
	}
}

func TestRenderNodeTable_WithNodes(t *testing.T) {
	nodes := []cluster.NodeInfo{
		{Hostname: "cp-1", MachineType: "controlplane"},
		{Hostname: "worker-1", MachineType: "worker"},
	}
	mem := map[string]memStats{
		"cp-1":     {TotalKB: 8000000, AvailableKB: 5000000},
		"worker-1": {TotalKB: 16000000, AvailableKB: 6000000},
	}

	result := RenderNodeTable(nodes, nil, mem, 80, 10)
	if !strings.Contains(result, "cp-1") {
		t.Error("expected cp-1 in output")
	}
	if !strings.Contains(result, "worker-1") {
		t.Error("expected worker-1 in output")
	}
	if !strings.Contains(result, "CP") {
		t.Error("expected CP type in output")
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

	// etcd should show status for CP, dash for worker
	if !strings.Contains(result, "etcd") {
		t.Error("expected etcd in service matrix")
	}
	// The "-" should appear for worker on etcd and trustd rows
	if !strings.Contains(result, "-") {
		t.Error("expected '-' for services not expected on worker")
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
	// Truncated to 4 chars + ellipsis rune
	if len([]rune(got)) > 5 {
		t.Errorf("truncate should limit to 5 runes, got %q (runes=%d)", got, len([]rune(got)))
	}
	if got := truncate("hi", 10); got != "hi" {
		t.Errorf("truncate should not change short strings, got %q", got)
	}
}
