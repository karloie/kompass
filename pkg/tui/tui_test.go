package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMainPgDownMovesByVisiblePage(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = make([]Row, 60)
	m.height = 20
	m.footerHeight = 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pgdown")})
	m1 := updated.(Model)

	// rowsHeight=18 after header/footer, page step keeps one-row overlap => 17
	if m1.cursorByPane[0] != 17 {
		t.Fatalf("expected pgdown to move cursor to 17, got %d", m1.cursorByPane[0])
	}
}

func TestMainPgUpMovesByVisiblePage(t *testing.T) {
	m := newRun(Options{Mode: ModeSelector})
	m.rowsByPane[0] = make([]Row, 60)
	m.height = 20
	m.footerHeight = 1
	m.cursorByPane[0] = 30

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pgup")})
	m1 := updated.(Model)

	if m1.cursorByPane[0] != 13 {
		t.Fatalf("expected pgup to move cursor to 13, got %d", m1.cursorByPane[0])
	}
}
