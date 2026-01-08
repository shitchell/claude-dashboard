package ui

import (
	"testing"
)

// TestDefaultKeyMap verifies the default key map is properly configured.
func TestDefaultKeyMap(t *testing.T) {
	km := DefaultKeyMap()

	// Verify navigation keys have both vim and arrow key bindings
	upKeys := km.Up.Keys()
	if len(upKeys) < 2 {
		t.Error("expected Up to have multiple key bindings")
	}

	downKeys := km.Down.Keys()
	if len(downKeys) < 2 {
		t.Error("expected Down to have multiple key bindings")
	}

	// Verify quit has both 'q' and 'ctrl+c'
	quitKeys := km.Quit.Keys()
	if len(quitKeys) < 2 {
		t.Error("expected Quit to have multiple key bindings")
	}

	// Verify help text is set
	if km.Up.Help().Key == "" {
		t.Error("expected Up to have help key")
	}
	if km.Up.Help().Desc == "" {
		t.Error("expected Up to have help description")
	}
}

// TestKeyMapShortHelp verifies ShortHelp returns expected bindings.
func TestKeyMapShortHelp(t *testing.T) {
	km := DefaultKeyMap()
	shortHelp := km.ShortHelp()

	if len(shortHelp) == 0 {
		t.Error("expected ShortHelp to return bindings")
	}

	// Should include at least navigation and quit
	if len(shortHelp) < 3 {
		t.Errorf("expected at least 3 short help bindings, got %d", len(shortHelp))
	}
}

// TestKeyMapFullHelp verifies FullHelp returns organized groups.
func TestKeyMapFullHelp(t *testing.T) {
	km := DefaultKeyMap()
	fullHelp := km.FullHelp()

	if len(fullHelp) == 0 {
		t.Error("expected FullHelp to return binding groups")
	}

	// Should have multiple groups
	if len(fullHelp) < 3 {
		t.Errorf("expected at least 3 help groups, got %d", len(fullHelp))
	}

	// Each group should have bindings
	for i, group := range fullHelp {
		if len(group) == 0 {
			t.Errorf("expected group %d to have bindings", i)
		}
	}
}

// TestKeyBindingsExist verifies all expected key bindings are configured.
func TestKeyBindingsExist(t *testing.T) {
	km := DefaultKeyMap()

	bindings := []struct {
		name    string
		binding interface{ Keys() []string }
	}{
		{"Up", km.Up},
		{"Down", km.Down},
		{"Left", km.Left},
		{"Right", km.Right},
		{"PageUp", km.PageUp},
		{"PageDown", km.PageDown},
		{"Home", km.Home},
		{"End", km.End},
		{"Enter", km.Enter},
		{"Refresh", km.Refresh},
		{"Sort", km.Sort},
		{"Search", km.Search},
		{"Filter", km.Filter},
		{"Help", km.Help},
		{"ToggleView", km.ToggleView},
		{"Quit", km.Quit},
		{"Escape", km.Escape},
	}

	for _, b := range bindings {
		t.Run(b.name, func(t *testing.T) {
			keys := b.binding.Keys()
			if len(keys) == 0 {
				t.Errorf("expected %s to have at least one key binding", b.name)
			}
		})
	}
}
