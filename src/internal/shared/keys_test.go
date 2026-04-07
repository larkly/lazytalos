package shared

import "testing"

func TestNewKeyBindingsInitialized(t *testing.T) {
	bindings := map[string]interface {
		Keys() []string
	}{
		"Tab5":             Keys.Tab5,
		"Tab6":             Keys.Tab6,
		"Tab7":             Keys.Tab7,
		"Tab8":             Keys.Tab8,
		"SubTabPrev":       Keys.SubTabPrev,
		"SubTabNext":       Keys.SubTabNext,
		"EditConfig":       Keys.EditConfig,
		"ContainerLogs":    Keys.ContainerLogs,
		"RemoveEtcdMember": Keys.RemoveEtcdMember,
	}

	for name, b := range bindings {
		if len(b.Keys()) == 0 {
			t.Errorf("Keys.%s has no keys bound (expected non-empty Keys() slice)", name)
		}
	}
}
