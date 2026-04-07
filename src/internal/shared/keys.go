package shared

import "charm.land/bubbles/v2/key"

// KeyMap holds all global key bindings for lazytalos.
type KeyMap struct {
	// Navigation
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Left     key.Binding
	Right    key.Binding
	Enter    key.Binding
	Back     key.Binding // Esc
	Tab      key.Binding
	ShiftTab key.Binding

	// Tab switching
	Tab1 key.Binding
	Tab2 key.Binding
	Tab3 key.Binding
	Tab4 key.Binding
	Tab5 key.Binding
	Tab6 key.Binding
	Tab7 key.Binding
	Tab8 key.Binding

	// Sub-tab navigation
	SubTabPrev key.Binding
	SubTabNext key.Binding

	// Selection
	Select    key.Binding // Space
	SelectAll key.Binding // A

	// Actions
	Filter        key.Binding // /
	Sort          key.Binding // s
	ContextPicker key.Binding // C
	Help          key.Binding // ?
	Quit          key.Binding // q
	Refresh       key.Binding // ctrl+r
	LogFollow     key.Binding // F
	GroupToggle   key.Binding // g (services tab)

	// Write actions (Ctrl-prefixed)
	Reboot           key.Binding // ctrl+o
	Shutdown         key.Binding // ctrl+d
	ServiceRestart   key.Binding // ctrl+k
	EditConfig       key.Binding // ctrl+e
	ContainerLogs    key.Binding // ctrl+l
	RemoveEtcdMember key.Binding // ctrl+m

	// Confirmation
	Confirm key.Binding // ctrl+s
	// Deny shares the same key as Back (Esc); in modal context use Deny, in navigation context use Back to indicate intent.
	Deny key.Binding // Esc (same as Back — handled contextually)
}

// Keys is the global key binding map.
var Keys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdown", "page down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "right"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next"),
	),
	ShiftTab: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev"),
	),
	Tab1: key.NewBinding(
		key.WithKeys("1"),
		key.WithHelp("1", "tab 1"),
	),
	Tab2: key.NewBinding(
		key.WithKeys("2"),
		key.WithHelp("2", "tab 2"),
	),
	Tab3: key.NewBinding(
		key.WithKeys("3"),
		key.WithHelp("3", "tab 3"),
	),
	Tab4: key.NewBinding(
		key.WithKeys("4"),
		key.WithHelp("4", "tab 4"),
	),
	Tab5: key.NewBinding(
		key.WithKeys("5"),
		key.WithHelp("5", "Containers"),
	),
	Tab6: key.NewBinding(
		key.WithKeys("6"),
		key.WithHelp("6", "Network"),
	),
	Tab7: key.NewBinding(
		key.WithKeys("7"),
		key.WithHelp("7", "Storage"),
	),
	Tab8: key.NewBinding(
		key.WithKeys("8"),
		key.WithHelp("8", "etcd"),
	),
	SubTabPrev: key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "previous sub-tab"),
	),
	SubTabNext: key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "next sub-tab"),
	),
	Select: key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("space", "select"),
	),
	SelectAll: key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("A", "select all"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Sort: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "sort"),
	),
	ContextPicker: key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("C", "switch context"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "refresh"),
	),
	LogFollow: key.NewBinding(
		key.WithKeys("F"),
		key.WithHelp("F", "follow logs"),
	),
	GroupToggle: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "group toggle"),
	),
	Reboot: key.NewBinding(
		key.WithKeys("ctrl+o"),
		key.WithHelp("ctrl+o", "reboot"),
	),
	Shutdown: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "shutdown"),
	),
	ServiceRestart: key.NewBinding(
		key.WithKeys("ctrl+k"),
		key.WithHelp("ctrl+k", "restart service"),
	),
	EditConfig: key.NewBinding(
		key.WithKeys("ctrl+e"),
		key.WithHelp("ctrl+e", "edit node config"),
	),
	ContainerLogs: key.NewBinding(
		key.WithKeys("ctrl+l"),
		key.WithHelp("ctrl+l", "view container logs"),
	),
	RemoveEtcdMember: key.NewBinding(
		key.WithKeys("ctrl+m"),
		key.WithHelp("ctrl+m", "remove etcd member"),
	),
	Confirm: key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", "confirm"),
	),
	Deny: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
}
