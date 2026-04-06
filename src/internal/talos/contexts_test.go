package talos

import (
	"os"
	"testing"
)

const testTalosconfig = `context: ctx1
contexts:
  ctx1:
    endpoints:
      - 10.0.0.1
    ca: ""
    crt: ""
    key: ""
  ctx2:
    endpoints:
      - 10.0.0.2
    ca: ""
    crt: ""
    key: ""
  ctx3:
    endpoints:
      - 10.0.0.3
    ca: ""
    crt: ""
    key: ""
`

func writeTempTalosconfig(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "talosconfig-*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	if _, err := f.WriteString(testTalosconfig); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing temp file: %v", err)
	}
	return f.Name()
}

func TestListContextNames_ReturnsSortedNames(t *testing.T) {
	path := writeTempTalosconfig(t)

	names, _, err := ListContextNames(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"ctx1", "ctx2", "ctx3"}
	if len(names) != len(want) {
		t.Fatalf("got %d names, want %d: %v", len(names), len(want), names)
	}
	for i, name := range names {
		if name != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, name, want[i])
		}
	}
}

func TestListContextNames_ReturnsCurrentContext(t *testing.T) {
	path := writeTempTalosconfig(t)

	_, current, err := ListContextNames(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if current != "ctx1" {
		t.Errorf("current context = %q, want %q", current, "ctx1")
	}
}

func TestListContextNames_ErrorOnNonexistentFile(t *testing.T) {
	_, _, err := ListContextNames("/nonexistent/path/talosconfig.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}
