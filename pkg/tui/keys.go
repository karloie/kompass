package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.view.SearchMode {
		return m.handleSearch(msg)
	}

	switch msg.String() {
	case "ctrl+c":
		return *m, tea.Quit
	case "esc", "q":
		m.view = nil
	case "enter":
		m.view = nil
	case "left", "h":
		m.view.ColScroll = maxInt(0, m.view.ColScroll-4)
	case "right", "l":
		m.view.ColScroll = minInt(m.maxColScroll(), m.view.ColScroll+4)
	case "home":
		m.view.ColScroll = 0
	case "end":
		m.view.ColScroll = m.maxColScroll()
	case "g":
		m.view.Scroll = 0
	case "G":
		m.view.Scroll = maxInt(0, len(m.view.Rows)-1)
	case "up", "k":
		if m.view.Scroll > 0 {
			m.view.Scroll--
		}
	case "down", "j":
		if m.view.Scroll < maxInt(0, len(m.view.Rows)-1) {
			m.view.Scroll++
		}
	case "pgup":
		m.view.Scroll = maxInt(0, m.view.Scroll-10)
	case "pgdown":
		m.view.Scroll = minInt(maxInt(0, len(m.view.Rows)-1), m.view.Scroll+10)
	case "/":
		m.view.SearchMode = true
		m.view.ActionStatus = ""
	case "y":
		if err := copyToClipboard(m.view.Raw); err != nil {
			m.view.ActionStatus = "copy failed: " + err.Error()
		} else {
			m.view.ActionStatus = "copied to clipboard"
		}
	case "e":
		return *m, openInEditorCmd(m.view.Raw)
	case "o":
		m.emitSelection = true
		return *m, tea.Quit
	}
	return *m, nil
}

func (m *Model) handleSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return *m, tea.Quit
	case tea.KeyCtrlU:
		m.view.SearchQuery = ""
	case tea.KeyEsc:
		m.view.SearchMode = false
	case tea.KeyEnter:
		m.view.SearchMode = false
		m.applySearch()
	case tea.KeyBackspace:
		if len(m.view.SearchQuery) > 0 {
			r := []rune(m.view.SearchQuery)
			m.view.SearchQuery = string(r[:len(r)-1])
		}
	default:
		if len(msg.Runes) > 0 {
			m.view.SearchQuery += string(msg.Runes)
		}
	}
	return *m, nil
}

func (m Model) currentRow() *Row {
	rows := m.rowsByPane[m.activePane]
	if len(rows) == 0 {
		return nil
	}
	idx := clamp(m.cursorByPane[m.activePane], 0, len(rows)-1)
	return &rows[idx]
}

func (m *Model) clearActiveSelection() bool {
	if len(m.selected[m.activePane]) == 0 {
		return false
	}
	m.selected[m.activePane] = map[string]bool{}
	return true
}

func (m Model) paneAvailable(pane int) bool {
	if pane < 0 || pane >= len(m.rowsByPane) {
		return false
	}
	if len(m.rowsByPane[pane]) > 0 {
		return true
	}
	// Keep pane 0 reachable when there is no data at all.
	if pane == 0 && len(m.rowsByPane[1]) == 0 {
		return true
	}
	return false
}

func (m Model) paneNext(direction int) int {
	if direction == 0 {
		direction = 1
	}
	current := m.activePane
	for i := 0; i < len(m.rowsByPane); i++ {
		candidate := (current + direction*(i+1) + len(m.rowsByPane)*2) % len(m.rowsByPane)
		if m.paneAvailable(candidate) {
			return candidate
		}
	}
	if m.paneAvailable(current) {
		return current
	}
	return 0
}

func (m *Model) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key != "up" && key != "k" && key != "down" && key != "j" {
		m.lastNavDir = 0
		m.navRepeat = 0
		m.navLastAt = time.Time{}
	}

	switch msg.String() {
	case "ctrl+c":
		return *m, tea.Quit
	case "esc":
		if m.clearActiveSelection() {
			return *m, nil
		}
		return *m, tea.Quit
	case "tab":
		if m.mode == ModeSelector {
			m.activePane = m.paneNext(1)
		}
	case "shift+tab":
		if m.mode == ModeSelector {
			m.activePane = m.paneNext(-1)
		}
	case "1":
		if m.paneAvailable(0) {
			m.activePane = 0
		} else if m.paneAvailable(1) {
			m.activePane = 1
		}
	case "2":
		if m.mode == ModeSelector {
			if m.paneAvailable(1) {
				m.activePane = 1
			} else if m.paneAvailable(0) {
				m.activePane = 0
			}
		}
	case "up", "k":
		now := time.Now()
		if m.now != nil {
			now = m.now()
		}
		step := 1
		if m.lastNavDir == -1 && m.navRepeat == 1 {
			delta := now.Sub(m.navLastAt)
			if delta >= doubleTapMin && delta <= doubleTapMax {
				step = m.navJumpStep()
				m.navRepeat = 0
			} else {
				m.navRepeat = 1
			}
		} else {
			m.navRepeat = 1
		}
		m.lastNavDir = -1
		m.navLastAt = now
		m.cursorByPane[m.activePane] = maxInt(0, m.cursorByPane[m.activePane]-step)
	case "down", "j":
		now := time.Now()
		if m.now != nil {
			now = m.now()
		}
		step := 1
		if m.lastNavDir == 1 && m.navRepeat == 1 {
			delta := now.Sub(m.navLastAt)
			if delta >= doubleTapMin && delta <= doubleTapMax {
				step = m.navJumpStep()
				m.navRepeat = 0
			} else {
				m.navRepeat = 1
			}
		} else {
			m.navRepeat = 1
		}
		m.lastNavDir = 1
		m.navLastAt = now
		maxCursor := maxInt(0, len(m.rowsByPane[m.activePane])-1)
		m.cursorByPane[m.activePane] = minInt(maxCursor, m.cursorByPane[m.activePane]+step)
	case "pgup":
		m.cursorByPane[m.activePane] = maxInt(0, m.cursorByPane[m.activePane]-m.navPageStep())
	case "pgdown":
		maxCursor := maxInt(0, len(m.rowsByPane[m.activePane])-1)
		m.cursorByPane[m.activePane] = minInt(maxCursor, m.cursorByPane[m.activePane]+m.navPageStep())
	case " ":
		if r := m.currentRow(); r != nil {
			if m.selected[m.activePane][r.Key] {
				delete(m.selected[m.activePane], r.Key)
			} else {
				m.selected[m.activePane][r.Key] = true
			}
		}
	case "ctrl+a":
		for _, r := range m.rowsByPane[m.activePane] {
			m.selected[m.activePane][r.Key] = true
		}
	case "+", "=":
		maxHeight := maxInt(1, m.height/3)
		m.footerHeight = minInt(maxHeight, m.footerHeight+1)
	case "-":
		m.footerHeight = maxInt(1, m.footerHeight-1)
	case "enter":
		if r := m.currentRow(); r != nil {
			m.view = viewYaml(*r, m.resources[r.Key])
		}
	case "o":
		m.emitSelection = true
		return *m, tea.Quit
	case "?":
		m.view = viewHelp()
	}
	return *m, nil
}

func (m Model) rowsHeight() int {
	return maxInt(1, m.height-1-m.footerHeight)
}

func (m Model) navPageStep() int {
	return maxInt(1, m.rowsHeight()-1)
}

func (m Model) navJumpStep() int {
	return maxInt(1, m.rowsHeight()/2)
}
