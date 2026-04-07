package node

import "testing"

func TestShortImage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "registry.k8s.io/pause:3.9",
			want:  "pause:3.9",
		},
		{
			input: "registry.k8s.io/pause@sha256:abc123def456",
			want:  "pause",
		},
		{
			input: "docker.io/library/nginx:latest",
			want:  "nginx:latest",
		},
		{
			input: "ghcr.io/siderolabs/flannel:v0.22.0@sha256:deadbeef",
			want:  "flannel:v0.22.0",
		},
		{
			input: "alpine",
			want:  "alpine",
		},
		{
			input: "",
			want:  "",
		},
	}

	for _, tc := range tests {
		got := shortImage(tc.input)
		if got != tc.want {
			t.Errorf("shortImage(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestContainerStruct is a compilation test ensuring the Container struct
// has the expected fields with the expected types.
func TestContainerStruct(t *testing.T) {
	c := Container{
		NodeHostname: "node-1",
		Namespace:    "k8s.io",
		Name:         "pause",
		Image:        "pause:3.9",
		FullImage:    "registry.k8s.io/pause:3.9",
		State:        "CONTAINER_RUNNING",
		PID:          1234,
		Status:       "CONTAINER_RUNNING",
	}
	if c.NodeHostname != "node-1" {
		t.Errorf("unexpected NodeHostname: %q", c.NodeHostname)
	}
	if c.PID != 1234 {
		t.Errorf("unexpected PID: %d", c.PID)
	}
}
