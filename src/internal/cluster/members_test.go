package cluster

import "testing"

func TestParseTalosVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Talos (v1.12.4)", "v1.12.4"},
		{"Talos (v1.13.0-alpha.0)", "v1.13.0-alpha.0"},
		{"Talos (v2.0.0)", "v2.0.0"},
		{"SomeOtherOS", "SomeOtherOS"},
		{"", ""},
		{"Talos ()", "Talos ()"},
	}

	for _, tt := range tests {
		got := ParseTalosVersion(tt.input)
		if got != tt.want {
			t.Errorf("ParseTalosVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNodeInfo_IsControlPlane(t *testing.T) {
	tests := []struct {
		machineType string
		want        bool
	}{
		{"controlplane", true},
		{"init", true},
		{"worker", false},
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		n := NodeInfo{MachineType: tt.machineType}
		if got := n.IsControlPlane(); got != tt.want {
			t.Errorf("NodeInfo{MachineType: %q}.IsControlPlane() = %v, want %v", tt.machineType, got, tt.want)
		}
	}
}
