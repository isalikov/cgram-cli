package tui

import "github.com/charmbracelet/bubbletea"

type keyMap struct {
	Tab        tea.Key
	Enter      tea.Key
	ShiftEnter tea.Key
	Up         tea.Key
	Down       tea.Key
	Esc        tea.Key
	Help       tea.Key
	Colon      tea.Key
	CtrlC      tea.Key
	J          tea.Key
	K          tea.Key
}

var Keys = keyMap{
	Tab:        tea.Key{Type: tea.KeyTab},
	Enter:      tea.Key{Type: tea.KeyEnter},
	ShiftEnter: tea.Key{Type: tea.KeyEnter},
	Up:         tea.Key{Type: tea.KeyUp},
	Down:       tea.Key{Type: tea.KeyDown},
	Esc:        tea.Key{Type: tea.KeyEscape},
	Help:       tea.Key{Type: tea.KeyRunes, Runes: []rune{'?'}},
	Colon:      tea.Key{Type: tea.KeyRunes, Runes: []rune{':'}},
	CtrlC:      tea.Key{Type: tea.KeyCtrlC},
}

func isKey(msg tea.KeyMsg, keys ...string) bool {
	for _, k := range keys {
		if msg.String() == k {
			return true
		}
	}
	return false
}
