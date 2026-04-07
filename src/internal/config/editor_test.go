package config

import (
	"testing"
)

func TestApplyModeConstants(t *testing.T) {
	tests := []struct {
		mode     ApplyMode
		expected string
	}{
		{ApplyModeReboot, "reboot"},
		{ApplyModeNoReboot, "no-reboot"},
		{ApplyModeStaged, "staged"},
	}

	for _, tt := range tests {
		if string(tt.mode) != tt.expected {
			t.Errorf("ApplyMode %q: got %q, want %q", tt.mode, string(tt.mode), tt.expected)
		}
	}
}

func TestParseSyntaxErrors_ValidYAML(t *testing.T) {
	validYAML := `
machine:
  type: controlplane
  network:
    hostname: talos-node-1
`
	errs := parseSyntaxErrors(validYAML)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid YAML, got: %v", errs)
	}
}

func TestParseSyntaxErrors_InvalidYAML(t *testing.T) {
	invalidYAML := `
machine:
  type: [unclosed bracket
  bad: : colon
`
	errs := parseSyntaxErrors(invalidYAML)
	if len(errs) == 0 {
		t.Error("expected errors for invalid YAML, got none")
	}
}

func TestParseSyntaxErrors_EmptyString(t *testing.T) {
	errs := parseSyntaxErrors("")
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty string, got: %v", errs)
	}
}

func TestValidateConfig_InvalidYAML(t *testing.T) {
	// ValidateConfig with nil client should still catch syntax errors
	// without making any network calls (syntax check happens first).
	errs, err := ValidateConfig(nil, nil, "node1", "bad: : yaml: [unclosed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) == 0 {
		t.Error("expected validation errors for invalid YAML, got none")
	}
}

func TestValidateConfig_ValidYAML(t *testing.T) {
	// ValidateConfig with nil client should return no errors for valid YAML.
	errs, err := ValidateConfig(nil, nil, "node1", "machine:\n  type: worker\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid YAML, got: %v", errs)
	}
}
