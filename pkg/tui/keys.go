package tui

import (
	"strings"

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
		m.view.Scroll = m.maxViewScroll()
	case "up", "k":
		if m.view.Scroll > 0 {
			m.view.Scroll--
		}
	case "down", "j":
		if m.view.Scroll < m.maxViewScroll() {
			m.view.Scroll++
		}
	case "pgup":
		m.view.Scroll = maxInt(0, m.view.Scroll-10)
	case "pgdown":
		m.view.Scroll = minInt(m.maxViewScroll(), m.view.Scroll+10)
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

func (m Model) maxViewScroll() int {
	if m.view == nil {
		return 0
	}
	rowsHeight := maxInt(1, m.height-2)
	return maxInt(0, len(m.view.Rows)-rowsHeight)
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
	if rows[idx].Separator {
		return nil
	}
	return &rows[idx]
}

func (m *Model) moveCursor(direction, step int) {
	rows := m.rowsByPane[m.activePane]
	if len(rows) == 0 {
		m.cursorByPane[m.activePane] = 0
		return
	}

	if step <= 0 {
		step = 1
	}
	if direction == 0 {
		direction = 1
	}

	current := clamp(m.cursorByPane[m.activePane], 0, len(rows)-1)
	target := clamp(current+direction*step, 0, len(rows)-1)

	if direction > 0 {
		for i := target; i < len(rows); i++ {
			if !rows[i].Separator {
				m.cursorByPane[m.activePane] = i
				return
			}
		}
		for i := target; i > current; i-- {
			if !rows[i].Separator {
				m.cursorByPane[m.activePane] = i
				return
			}
		}
		return
	}

	for i := target; i >= 0; i-- {
		if !rows[i].Separator {
			m.cursorByPane[m.activePane] = i
			return
		}
	}
	for i := target; i < current; i++ {
		if !rows[i].Separator {
			m.cursorByPane[m.activePane] = i
			return
		}
	}
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
	if m.filterMode {
		return m.handleFilterInput(msg)
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
		m.jumpRoot(1)
	case "shift+tab":
		m.jumpRoot(-1)
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
		m.moveCursor(-1, 1)
	case "down", "j":
		m.moveCursor(1, 1)
	case "pgup":
		m.moveCursor(-1, m.navPageStep())
	case "pgdown":
		m.moveCursor(1, m.navPageStep())
	case " ":
		if r := m.currentRow(); m.canDescribeRow(r) {
			if m.selected[m.activePane][r.Key] {
				delete(m.selected[m.activePane], r.Key)
			} else {
				m.selected[m.activePane][r.Key] = true
			}
		}
	case "ctrl+a":
		for _, r := range m.rowsByPane[m.activePane] {
			if r.Separator {
				continue
			}
			if !m.canDescribeRow(&r) {
				continue
			}
			m.selected[m.activePane][r.Key] = true
		}
	case "+", "=":
		maxHeight := maxInt(1, m.height/3)
		m.footerHeight = minInt(maxHeight, m.footerHeight+1)
	case "-":
		m.footerHeight = maxInt(1, m.footerHeight-1)
	case "enter":
		if r := m.currentRow(); m.canDescribeRow(r) {
			m.view = viewDescribe(*r, m.context, m.namespace)
		}
	case "o":
		m.emitSelection = true
		return *m, tea.Quit
	case "?":
		m.view = viewHelp()
	case "f":
		m.filterMode = true
	}
	return *m, nil
}

func (m Model) canDescribeRow(r *Row) bool {
	if r == nil {
		return false
	}
	if m.mode != ModeSelector {
		return false
	}
	if len(m.resources) == 0 {
		return false
	}
	_, ok := m.resources[r.Key]
	return ok
}

func (m *Model) jumpRoot(direction int) {
	rows := m.rowsByPane[m.activePane]
	if len(rows) == 0 {
		return
	}
	if direction == 0 {
		direction = 1
	}

	current := clamp(m.cursorByPane[m.activePane], 0, len(rows)-1)
	if direction > 0 {
		for i := current + 1; i < len(rows); i++ {
			if rows[i].Separator {
				continue
			}
			if rows[i].Depth == 0 {
				m.cursorByPane[m.activePane] = i
				return
			}
		}
		for i := 0; i <= current; i++ {
			if rows[i].Separator {
				continue
			}
			if rows[i].Depth == 0 {
				m.cursorByPane[m.activePane] = i
				return
			}
		}
		return
	}

	for i := current - 1; i >= 0; i-- {
		if rows[i].Separator {
			continue
		}
		if rows[i].Depth == 0 {
			m.cursorByPane[m.activePane] = i
			return
		}
	}
	for i := len(rows) - 1; i >= current; i-- {
		if rows[i].Separator {
			continue
		}
		if rows[i].Depth == 0 {
			m.cursorByPane[m.activePane] = i
			return
		}
	}
}

func (m *Model) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return *m, tea.Quit
	case tea.KeyEsc:
		m.filterMode = false
		return *m, nil
	case tea.KeyEnter:
		m.filterMode = false
		m.applyMainFilter()
		return *m, nil
	case tea.KeyCtrlU:
		m.filterQuery = ""
		m.applyMainFilter()
		return *m, nil
	case tea.KeyBackspace:
		if len(m.filterQuery) > 0 {
			r := []rune(m.filterQuery)
			m.filterQuery = string(r[:len(r)-1])
			m.applyMainFilter()
		}
		return *m, nil
	default:
		if len(msg.Runes) > 0 {
			m.filterQuery += string(msg.Runes)
			m.applyMainFilter()
		}
		return *m, nil
	}
}

func (m *Model) applyMainFilter() {
	if m.mode == ModeSelector && m.sourceTrees != nil {
		if strings.TrimSpace(m.filterQuery) == "" {
			m.rowsByPane[0] = m.allRowsByPane[0]
			m.rowsByPane[1] = m.allRowsByPane[1]
		} else {
			matcher := buildQueryMatcher(m.filterQuery)
			filteredTrees := filterResponseTrees(m.sourceTrees, matcher)
			rows := flattenTrees(filteredTrees)
			m.rowsByPane[0] = rows
			m.rowsByPane[1] = singleRows(rows)
		}

		m.normalizeCursor(0)
		m.normalizeCursor(1)
		for pane := range m.selected {
			for key := range m.selected[pane] {
				if !m.rowKeyVisible(pane, key) {
					delete(m.selected[pane], key)
				}
			}
		}
		return
	}

	for pane := range m.rowsByPane {
		source := m.allRowsByPane[pane]
		if strings.TrimSpace(m.filterQuery) == "" {
			m.rowsByPane[pane] = source
			m.normalizeCursor(pane)
			continue
		}

		matcher := buildQueryMatcher(m.filterQuery)
		filtered := make([]Row, 0, len(source))
		for _, row := range source {
			if row.Separator {
				continue
			}
			if matcher.test(rowSearchText(row)) {
				filtered = append(filtered, row)
			}
		}
		m.rowsByPane[pane] = filtered
		m.normalizeCursor(pane)
	}

	for pane := range m.selected {
		for key := range m.selected[pane] {
			if !m.rowKeyVisible(pane, key) {
				delete(m.selected[pane], key)
			}
		}
	}
}

func (m *Model) normalizeCursor(pane int) {
	rows := m.rowsByPane[pane]
	if len(rows) == 0 {
		m.cursorByPane[pane] = 0
		return
	}

	idx := clamp(m.cursorByPane[pane], 0, len(rows)-1)
	if !rows[idx].Separator {
		m.cursorByPane[pane] = idx
		return
	}

	for i := idx + 1; i < len(rows); i++ {
		if !rows[i].Separator {
			m.cursorByPane[pane] = i
			return
		}
	}
	for i := idx - 1; i >= 0; i-- {
		if !rows[i].Separator {
			m.cursorByPane[pane] = i
			return
		}
	}
	m.cursorByPane[pane] = 0
}

func (m Model) rowKeyVisible(pane int, key string) bool {
	for _, row := range m.rowsByPane[pane] {
		if row.Key == key {
			return true
		}
	}
	return false
}

func (m Model) rowsHeight() int {
	return maxInt(1, m.height-1-m.footerHeight)
}

func (m Model) navPageStep() int {
	return maxInt(1, m.rowsHeight()-1)
}
