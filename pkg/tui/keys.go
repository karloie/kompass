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
		m.closeView()
	case "enter":
		m.closeView()
	case "left", "h":
		m.panView(-4)
	case "right", "l":
		m.panView(4)
	case "home":
		m.panViewToStart()
	case "end":
		m.panViewToEnd()
	case "g":
		m.scrollViewToTop()
	case "G":
		m.scrollViewToBottom()
	case "up", "k":
		m.scrollView(-1)
	case "down", "j":
		m.scrollView(1)
	case "pgup":
		m.scrollView(-m.viewPageStep())
	case "pgdown":
		m.scrollView(m.viewPageStep())
	case "/":
		m.view.SearchMode = true
		m.view.ActionStatus = ""
	case "y":
		m.copyViewRaw()
	case "e":
		return *m, openInEditorCmd(m.view.Raw)
	case "o":
		m.emitSelection = true
		return *m, tea.Quit
	}
	return *m, nil
}

func (m *Model) closeView() {
	m.view = nil
}

func (m *Model) panView(delta int) {
	m.view.ColScroll = clamp(m.view.ColScroll+delta, 0, m.maxColScroll())
}

func (m *Model) panViewToStart() {
	m.view.ColScroll = 0
}

func (m *Model) panViewToEnd() {
	m.view.ColScroll = m.maxColScroll()
}

func (m *Model) scrollView(delta int) {
	m.view.Scroll = clamp(m.view.Scroll+delta, 0, m.maxViewScroll())
}

func (m *Model) scrollViewToTop() {
	m.view.Scroll = 0
}

func (m *Model) scrollViewToBottom() {
	m.view.Scroll = m.maxViewScroll()
}

func (m *Model) copyViewRaw() {
	if err := copyToClipboard(m.view.Raw); err != nil {
		m.view.ActionStatus = "copy failed: " + err.Error()
		return
	}
	m.view.ActionStatus = "copied to clipboard"
}

func (m Model) maxViewScroll() int {
	if m.view == nil {
		return 0
	}
	return maxInt(0, len(m.view.Rows)-m.viewRowsHeight())
}

func (m Model) viewPageStep() int {
	return maxInt(1, m.viewRowsHeight()-1)
}

func (m Model) viewRowsHeight() int {
	return maxInt(1, m.height-2)
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
		m.view.SearchQuery = trimLastRune(m.view.SearchQuery)
	default:
		m.view.SearchQuery = appendRunes(m.view.SearchQuery, msg.Runes)
	}
	return *m, nil
}

func trimLastRune(value string) string {
	if value == "" {
		return value
	}
	r := []rune(value)
	return string(r[:len(r)-1])
}

func appendRunes(value string, runes []rune) string {
	if len(runes) == 0 {
		return value
	}
	return value + string(runes)
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
		if next, ok := findNonSeparatorIndex(rows, target, len(rows)-1, 1); ok {
			m.cursorByPane[m.activePane] = next
			return
		}
		if next, ok := findNonSeparatorIndex(rows, target, current+1, -1); ok {
			m.cursorByPane[m.activePane] = next
			return
		}
		return
	}

	if next, ok := findNonSeparatorIndex(rows, target, 0, -1); ok {
		m.cursorByPane[m.activePane] = next
		return
	}
	if next, ok := findNonSeparatorIndex(rows, target, current-1, 1); ok {
		m.cursorByPane[m.activePane] = next
		return
	}
}

func findNonSeparatorIndex(rows []Row, start, end, direction int) (int, bool) {
	if direction == 0 || len(rows) == 0 {
		return 0, false
	}
	if direction > 0 {
		for i := start; i <= end && i < len(rows); i++ {
			if i >= 0 && !rows[i].Separator {
				return i, true
			}
		}
		return 0, false
	}
	for i := start; i >= end && i >= 0; i-- {
		if i < len(rows) && !rows[i].Separator {
			return i, true
		}
	}
	return 0, false
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
		m.toggleCurrentSelection()
	case "ctrl+a":
		m.selectAllDescribableRows()
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
	case "r":
		if cmd := m.startRefresh(); cmd != nil {
			return *m, cmd
		}
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
		if next, ok := findRootIndex(rows, current+1, len(rows)-1, 1); ok {
			m.cursorByPane[m.activePane] = next
			return
		}
		if next, ok := findRootIndex(rows, 0, current, 1); ok {
			m.cursorByPane[m.activePane] = next
			return
		}
		return
	}

	if next, ok := findRootIndex(rows, current-1, 0, -1); ok {
		m.cursorByPane[m.activePane] = next
		return
	}
	if next, ok := findRootIndex(rows, len(rows)-1, current, -1); ok {
		m.cursorByPane[m.activePane] = next
		return
	}
}

func findRootIndex(rows []Row, start, end, direction int) (int, bool) {
	if direction == 0 || len(rows) == 0 {
		return 0, false
	}
	if direction > 0 {
		for i := start; i <= end && i < len(rows); i++ {
			if i >= 0 && !rows[i].Separator && rows[i].Depth == 0 {
				return i, true
			}
		}
		return 0, false
	}
	for i := start; i >= end && i >= 0; i-- {
		if i < len(rows) && !rows[i].Separator && rows[i].Depth == 0 {
			return i, true
		}
	}
	return 0, false
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
		next := trimLastRune(m.filterQuery)
		if next != m.filterQuery {
			m.filterQuery = next
			m.applyMainFilter()
		}
		return *m, nil
	default:
		next := appendRunes(m.filterQuery, msg.Runes)
		if next != m.filterQuery {
			m.filterQuery = next
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
	height := m.height - 1 - m.footerHeight
	if m.mode == ModeSelector {
		height--
	}
	return maxInt(1, height)
}

func (m Model) navPageStep() int {
	return maxInt(1, m.rowsHeight()-1)
}

func (m *Model) toggleCurrentSelection() {
	r := m.currentRow()
	if !m.canDescribeRow(r) {
		return
	}
	if m.selected[m.activePane][r.Key] {
		delete(m.selected[m.activePane], r.Key)
		return
	}
	m.selected[m.activePane][r.Key] = true
}

func (m *Model) selectAllDescribableRows() {
	for _, r := range m.rowsByPane[m.activePane] {
		if r.Separator || !m.canDescribeRow(&r) {
			continue
		}
		m.selected[m.activePane][r.Key] = true
	}
}
