package ui

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines all key bindings for the application.
// Each binding supports multiple keys for the same action.
type KeyMap struct {
	// Navigation
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding

	// Page navigation
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding

	// Actions
	Enter   key.Binding
	Refresh key.Binding
	Sort    key.Binding
	Search  key.Binding
	Filter  key.Binding

	// View toggles
	Help       key.Binding
	ToggleView key.Binding

	// Exit
	Quit   key.Binding
	Escape key.Binding
}

// DefaultKeyMap returns the default key bindings for the application.
// These can be customized through configuration.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Navigation keys - support both vim and arrow keys
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("up/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("down/j", "move down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("left/h", "move left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("right/l", "move right"),
		),

		// Page navigation
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup/ctrl+u", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn/ctrl+d", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to first"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to last"),
		),

		// Actions
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select session"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Sort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "change sort"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),

		// View toggles
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		ToggleView: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle view"),
		),

		// Exit keys
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel/close"),
		),
	}
}

// ShortHelp returns key bindings for the short help view.
// These are the most commonly used key bindings.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up,
		k.Down,
		k.Enter,
		k.Refresh,
		k.Help,
		k.Quit,
	}
}

// FullHelp returns all key bindings organized into groups.
// This is used for the expanded help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Navigation group
		{k.Up, k.Down, k.Left, k.Right},
		// Page navigation group
		{k.PageUp, k.PageDown, k.Home, k.End},
		// Actions group
		{k.Enter, k.Refresh, k.Sort, k.Search, k.Filter},
		// View and exit group
		{k.Help, k.ToggleView, k.Quit, k.Escape},
	}
}
