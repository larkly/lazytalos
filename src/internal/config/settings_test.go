package config

import (
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	d := Defaults()
	if d.General.RefreshInterval != 5 {
		t.Errorf("default refresh = %d, want 5", d.General.RefreshInterval)
	}
	if d.Thresholds.MemoryWarning != 60 {
		t.Errorf("default memory warning = %d, want 60", d.Thresholds.MemoryWarning)
	}
	if d.Thresholds.MemoryCritical != 80 {
		t.Errorf("default memory critical = %d, want 80", d.Thresholds.MemoryCritical)
	}
	if d.Thresholds.CPUWarning != 70 {
		t.Errorf("default cpu warning = %d, want 70", d.Thresholds.CPUWarning)
	}
	if d.Colors.Primary != "#00BCD4" {
		t.Errorf("default primary = %s, want #00BCD4", d.Colors.Primary)
	}
	if d.Keybindings["settings"] != "ctrl+k" {
		t.Errorf("default settings binding = %s, want ctrl+k", d.Keybindings["settings"])
	}
	if d.Keybindings["service_restart"] != "ctrl+j" {
		t.Errorf("default service_restart binding = %s, want ctrl+j", d.Keybindings["service_restart"])
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := Defaults()
	cfg.General.RefreshInterval = 10
	cfg.Thresholds.MemoryWarning = 50
	cfg.Colors.Primary = "#FF0000"

	if err := cfg.SaveTo(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.General.RefreshInterval != 10 {
		t.Errorf("loaded refresh = %d, want 10", loaded.General.RefreshInterval)
	}
	if loaded.Thresholds.MemoryWarning != 50 {
		t.Errorf("loaded memory warning = %d, want 50", loaded.Thresholds.MemoryWarning)
	}
	if loaded.Colors.Primary != "#FF0000" {
		t.Errorf("loaded primary = %s, want #FF0000", loaded.Colors.Primary)
	}
	// Bool defaults should be preserved
	if loaded.General.CheckForUpdates != true {
		t.Error("expected check_for_updates to default to true")
	}
}

func TestLoadFrom_Missing(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// Should return defaults
	if cfg.General.RefreshInterval != 5 {
		t.Errorf("expected default refresh, got %d", cfg.General.RefreshInterval)
	}
}

func TestMergeWithDefaults(t *testing.T) {
	file := Config{
		General: GeneralConfig{
			RefreshInterval: 0, // zero — should get default
		},
		Colors: ColorConfig{
			Primary: "#AABBCC",
			// rest empty — should get defaults
		},
	}
	defaults := Defaults()
	merged := mergeWithDefaults(file, defaults)

	if merged.General.RefreshInterval != 5 {
		t.Errorf("expected default refresh 5, got %d", merged.General.RefreshInterval)
	}
	if merged.Colors.Primary != "#AABBCC" {
		t.Errorf("expected custom primary, got %s", merged.Colors.Primary)
	}
	if merged.Colors.Success != defaults.Colors.Success {
		t.Errorf("expected default success color, got %s", merged.Colors.Success)
	}
	if merged.Thresholds.MemoryWarning != defaults.Thresholds.MemoryWarning {
		t.Errorf("expected default memory warning, got %d", merged.Thresholds.MemoryWarning)
	}
}
