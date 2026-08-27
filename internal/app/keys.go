package app

import (
	"os"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
)

// textInputFocused reports whether any of the given text inputs has focus,
// meaning ordinary typing (e.g. "q") must reach the input instead of being
// interpreted as an app keybinding.
func textInputFocused(inputs ...textinput.Model) bool {
	for _, in := range inputs {
		if in.Focused() {
			return true
		}
	}
	return false
}

// filterActive reports whether a list's filter is engaged (being typed into
// or applied). While active, keys like q, space, enter and esc must be
// forwarded to the list instead of the screen's own bindings.
func filterActive(l list.Model) bool {
	return l.FilterState() != list.Unfiltered
}

// outputExists reports whether path already exists (used for overwrite
// confirmations; commands run with -y).
func outputExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
