package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/larkly/lazytalos/internal/shared"
	"gopkg.in/yaml.v3"
)

// Config is the full persisted application configuration.
type Config struct {
	General     GeneralConfig     `yaml:"general"`
	Colors      ColorConfig       `yaml:"colors"`
	Thresholds  ThresholdConfig   `yaml:"thresholds"`
	Keybindings map[string]string `yaml:"keybindings,omitempty"`
}

// GeneralConfig holds non-visual settings.
type GeneralConfig struct {
	RefreshInterval     int  `yaml:"refresh_interval"`
	PlainMode           bool `yaml:"plain_mode"`
	CheckForUpdates     bool `yaml:"check_for_updates"`
	UpdateCheckInterval int  `yaml:"update_check_interval"`
	AlwaysPickContext   bool `yaml:"always_pick_context"`
}

// ThresholdConfig holds resource warning/critical thresholds as percentages (0-100).
type ThresholdConfig struct {
	MemoryWarning  int `yaml:"memory_warning"`
	MemoryCritical int `yaml:"memory_critical"`
	CPUWarning     int `yaml:"cpu_warning"`
}

// ColorConfig holds hex color strings for the UI palette.
type ColorConfig struct {
	Primary   string `yaml:"primary"`
	Secondary string `yaml:"secondary"`
	Success   string `yaml:"success"`
	Warning   string `yaml:"warning"`
	Error     string `yaml:"error"`
	Muted     string `yaml:"muted"`
	Bg        string `yaml:"bg"`
	Fg        string `yaml:"fg"`
	Highlight string `yaml:"highlight"`
}

// DefaultPath returns ~/.config/lazytalos/config.yaml.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "lazytalos", "config.yaml")
}

// Defaults returns the hardcoded default configuration.
func Defaults() Config {
	return Config{
		General: GeneralConfig{
			RefreshInterval:     5,
			PlainMode:           false,
			CheckForUpdates:     true,
			UpdateCheckInterval: 24,
			AlwaysPickContext:   false,
		},
		Colors: ColorConfig{
			Primary:   "#00BCD4",
			Secondary: "#56B6C2",
			Success:   "#2AA198",
			Warning:   "#B58900",
			Error:     "#DC322F",
			Muted:     "#657B83",
			Bg:        "#002B36",
			Fg:        "#839496",
			Highlight: "#FDF6E3",
		},
		Thresholds: ThresholdConfig{
			MemoryWarning:  60,
			MemoryCritical: 80,
			CPUWarning:     70,
		},
		Keybindings: DefaultKeybindings(),
	}
}

// DefaultKeybindings returns the default key bindings map.
func DefaultKeybindings() map[string]string {
	return map[string]string{
		"quit":             "q,ctrl+c",
		"help":             "?",
		"settings":         "ctrl+k",
		"context_picker":   "C",
		"filter":           "/",
		"enter":            "enter",
		"back":             "esc",
		"up":               "up,k",
		"down":             "down,j",
		"left":             "left,h",
		"right":            "right,l",
		"tab":              "tab",
		"shift_tab":        "shift+tab",
		"page_up":          "pgup",
		"page_down":        "pgdown",
		"select":           "space",
		"select_all":       "A",
		"sort":             "s",
		"refresh":          "ctrl+r",
		"log_follow":       "F",
		"group_toggle":     "g",
		"reboot":           "ctrl+o",
		"shutdown":         "ctrl+d",
		"service_restart":  "ctrl+j",
		"edit_config":      "ctrl+e",
		"container_logs":   "ctrl+l",
		"remove_etcd":      "ctrl+m",
		"confirm":          "ctrl+s",
		"upgrade_cluster":  "ctrl+u",
		"reset_node":       "ctrl+x",
		"pause_upgrade":    "ctrl+p",
		"abort_upgrade":    "ctrl+a",
		"yank_ip":          "y",
		"yank_endpoint":    "Y",
		"config_view":      "ctrl+,",
	}
}

// rawGeneral mirrors GeneralConfig with pointer bools to detect presence in YAML.
type rawGeneral struct {
	RefreshInterval     int   `yaml:"refresh_interval"`
	PlainMode           *bool `yaml:"plain_mode"`
	CheckForUpdates     *bool `yaml:"check_for_updates"`
	UpdateCheckInterval *int  `yaml:"update_check_interval"`
	AlwaysPickContext   *bool `yaml:"always_pick_context"`
}

type rawConfig struct {
	General     rawGeneral        `yaml:"general"`
	Colors      ColorConfig       `yaml:"colors"`
	Thresholds  ThresholdConfig   `yaml:"thresholds"`
	Keybindings map[string]string `yaml:"keybindings,omitempty"`
}

// Load reads config from DefaultPath. Returns Defaults() if file does not exist.
func Load() (Config, error) {
	return LoadFrom(DefaultPath())
}

// LoadFrom reads config from the given path.
func LoadFrom(path string) (Config, error) {
	defaults := Defaults()
	if path == "" {
		return defaults, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaults, nil
		}
		return defaults, err
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return defaults, err
	}

	file := Config{
		General: GeneralConfig{
			RefreshInterval: raw.General.RefreshInterval,
		},
		Colors:      raw.Colors,
		Thresholds:  raw.Thresholds,
		Keybindings: raw.Keybindings,
	}

	// Use raw pointer fields to distinguish "explicitly false" from "absent".
	if raw.General.PlainMode != nil {
		file.General.PlainMode = *raw.General.PlainMode
	} else {
		file.General.PlainMode = defaults.General.PlainMode
	}
	if raw.General.CheckForUpdates != nil {
		file.General.CheckForUpdates = *raw.General.CheckForUpdates
	} else {
		file.General.CheckForUpdates = defaults.General.CheckForUpdates
	}
	if raw.General.AlwaysPickContext != nil {
		file.General.AlwaysPickContext = *raw.General.AlwaysPickContext
	} else {
		file.General.AlwaysPickContext = defaults.General.AlwaysPickContext
	}
	if raw.General.UpdateCheckInterval != nil {
		file.General.UpdateCheckInterval = *raw.General.UpdateCheckInterval
	} else {
		file.General.UpdateCheckInterval = defaults.General.UpdateCheckInterval
	}

	return mergeWithDefaults(file, defaults), nil
}

// mergeWithDefaults fills zero-valued fields with defaults.
func mergeWithDefaults(file, defaults Config) Config {
	if file.General.RefreshInterval == 0 {
		file.General.RefreshInterval = defaults.General.RefreshInterval
	}
	if file.General.UpdateCheckInterval == 0 {
		file.General.UpdateCheckInterval = defaults.General.UpdateCheckInterval
	}

	// Thresholds
	if file.Thresholds.MemoryWarning == 0 {
		file.Thresholds.MemoryWarning = defaults.Thresholds.MemoryWarning
	}
	if file.Thresholds.MemoryCritical == 0 {
		file.Thresholds.MemoryCritical = defaults.Thresholds.MemoryCritical
	}
	if file.Thresholds.CPUWarning == 0 {
		file.Thresholds.CPUWarning = defaults.Thresholds.CPUWarning
	}

	// Colors
	if file.Colors.Primary == "" {
		file.Colors.Primary = defaults.Colors.Primary
	}
	if file.Colors.Secondary == "" {
		file.Colors.Secondary = defaults.Colors.Secondary
	}
	if file.Colors.Success == "" {
		file.Colors.Success = defaults.Colors.Success
	}
	if file.Colors.Warning == "" {
		file.Colors.Warning = defaults.Colors.Warning
	}
	if file.Colors.Error == "" {
		file.Colors.Error = defaults.Colors.Error
	}
	if file.Colors.Muted == "" {
		file.Colors.Muted = defaults.Colors.Muted
	}
	if file.Colors.Bg == "" {
		file.Colors.Bg = defaults.Colors.Bg
	}
	if file.Colors.Fg == "" {
		file.Colors.Fg = defaults.Colors.Fg
	}
	if file.Colors.Highlight == "" {
		file.Colors.Highlight = defaults.Colors.Highlight
	}

	// Keybindings
	if file.Keybindings == nil {
		file.Keybindings = defaults.Keybindings
	} else {
		for k, v := range defaults.Keybindings {
			if _, ok := file.Keybindings[k]; !ok {
				file.Keybindings[k] = v
			}
		}
	}

	return file
}

// Save writes config to DefaultPath, creating directories as needed.
func (c *Config) Save() error {
	return c.SaveTo(DefaultPath())
}

// SaveTo writes config to the given path.
func (c *Config) SaveTo(path string) error {
	shared.Debugf("[config] SaveTo: path=%s", path)
	if path == "" {
		return errors.New("config: empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
