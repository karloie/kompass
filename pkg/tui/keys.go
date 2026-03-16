package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	filterDebounceDuration     = 100 * time.Millisecond
	filterDebounceRowThreshold = 500
)

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return *m, tea.Quit
	case "ctrl+t":
		m.themeName = cycleTheme()
	case "esc", "q":
		m.closeView()
	case "tab":
		m.cycleViewPage(1)
	case "shift+tab":
		m.cycleViewPage(-1)
	case "enter":
		m.closeView()
	case "left", "h":
		m.panView(-4)
	case "right", "l":
		m.panView(4)
	case "home":
		m.scrollViewToTop()
	case "end":
		m.scrollViewToBottom()
	case "up", "k":
		m.scrollView(-1)
	case "down", "j":
		m.scrollView(1)
	case "pgup":
		m.scrollView(-m.viewPageStep())
	case "pgdown":
		m.scrollView(m.viewPageStep())
	case "o":
		m.emitSelection = true
		return *m, tea.Quit
	}
	return *m, nil
}

func (m *Model) closeView() {
	if m.view != nil {
		m.view.syncActivePage()
	}
	m.view = nil
}

func (m *Model) cycleViewPage(step int) {
	if m.view == nil {
		return
	}
	m.view.cyclePage(step)
}

func (m *Model) panView(delta int) {
	m.view.ColScroll = clamp(m.view.ColScroll+delta, 0, m.maxColScroll())
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
	overhead := 3 // header + command bar + footer
	if m.view != nil && m.view.Kind == FileHelp {
		overhead = 2 // no command bar for help
	}
	return maxInt(1, m.height-overhead)
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
	if msg.String() == "ctrl+c" {
		return *m, tea.Quit
	}

	if m.submode == SubmodeConfirmQuit {
		switch msg.String() {
		case "enter", "y":
			return *m, tea.Quit
		case "esc", "n":
			m.submode = SubmodeNone
			return *m, nil
		default:
			return *m, nil
		}
	}

	if m.submode == SubmodeContextList {
		return m.handleListPickerInput(&m.contextList, func(selected string) {
			m.context = selected
		}, msg)
	}

	if m.submode == SubmodeNamespaceList {
		return m.handleListPickerInput(&m.namespaceList, func(selected string) {
			m.namespace = selected
		}, msg)
	}

	if m.submode == SubmodeFilter {
		return m.handleFilterInput(msg)
	}

	switch msg.String() {
	case "esc":
		if m.clearActiveSelection() {
			return *m, nil
		}
		m.submode = SubmodeConfirmQuit
		return *m, nil
	case "ctrl+t":
		m.themeName = cycleTheme()
	case "c":
		return *m, m.openContextList()
	case "n":
		return *m, m.openNamespaceList()
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
	case "left", "h":
		m.panMain(-4)
	case "right", "l":
		m.panMain(4)
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
			action := m.effectiveAction(r)
			target := resourceViewTarget(*r, m.namespace)
			v, cmd := openResourceViewAsync(target, m.context, m.resources, m.netpolProvider, m.hubbleProvider, action)
			m.view = v
			if cmd != nil {
				return *m, cmd
			}
		}
	case "ctrl+enter":
		if r := m.currentRow(); m.canDescribeRow(r) {
			actions := m.rowAvailableActions(r)
			if len(actions) > 0 {
				idx := 0
				for i, a := range actions {
					if a == m.selectedAction {
						idx = i
						break
					}
				}
				m.selectedAction = actions[(idx+1)%len(actions)]
			}
		}
	case "o":
		m.emitSelection = true
		return *m, tea.Quit
	case "?":
		m.view = viewHelp()
	case "f", "/":
		m.openFilterModal()
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
		m.filterQuery = m.filterSaved
		m.submode = SubmodeNone
		m.applyMainFilter()
		return *m, nil
	case tea.KeyEnter:
		m.submode = SubmodeNone
		return *m, nil
	case tea.KeyCtrlL:
		m.filterQuery = ""
		m.filterSaved = ""
		m.submode = SubmodeNone
		m.applyMainFilter()
		return *m, nil
	case tea.KeyCtrlU:
		m.filterQuery = ""
		if cmd := m.scheduleFilterApply(); cmd != nil {
			return *m, cmd
		}
		return *m, nil
	case tea.KeyBackspace:
		next := trimLastRune(m.filterQuery)
		if next != m.filterQuery {
			m.filterQuery = next
			if cmd := m.scheduleFilterApply(); cmd != nil {
				return *m, cmd
			}
		}
		return *m, nil
	default:
		next := appendRunes(m.filterQuery, msg.Runes)
		if next != m.filterQuery {
			m.filterQuery = next
			if cmd := m.scheduleFilterApply(); cmd != nil {
				return *m, cmd
			}
		}
		return *m, nil
	}
}

func (m *Model) shouldDebounceFilter() bool {
	if m.mode != ModeSelector || m.sourceTrees == nil {
		return false
	}
	return len(m.allRowsByPane[0]) >= filterDebounceRowThreshold
}

func (m *Model) scheduleFilterApply() tea.Cmd {
	if !m.shouldDebounceFilter() {
		m.applyMainFilter()
		return nil
	}
	query := m.filterQuery
	return tea.Tick(filterDebounceDuration, func(time.Time) tea.Msg {
		return filterApplyMsg{query: query}
	})
}

func (m *Model) openFilterModal() {
	m.filterSaved = m.filterQuery
	m.submode = SubmodeFilter
}

func (m *Model) openContextList() tea.Cmd {
	m.submode = SubmodeContextList
	m.contextList = listPickerState{Loading: true}
	ctx := m.context
	return func() tea.Msg {
		options, err := listScopeOptions("context", ctx)
		return scopeListResultMsg{mode: "context", options: options, err: err}
	}
}

func (m *Model) openNamespaceList() tea.Cmd {
	m.submode = SubmodeNamespaceList
	m.namespaceList = listPickerState{Loading: true}
	ctx := m.context
	return func() tea.Msg {
		options, err := listScopeOptions("namespace", ctx)
		return scopeListResultMsg{mode: "namespace", options: options, err: err}
	}
}

func (m *Model) handleListPickerInput(state *listPickerState, apply func(string), msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return *m, tea.Quit
	case tea.KeyEsc:
		m.submode = SubmodeNone
		return *m, nil
	case tea.KeyUp:
		if len(state.Options) > 0 {
			if state.Index > 0 {
				state.Index--
			} else {
				state.Index = len(state.Options) - 1
			}
		}
		return *m, nil
	case tea.KeyDown:
		if len(state.Options) > 0 {
			state.Index = (state.Index + 1) % len(state.Options)
		}
		return *m, nil
	case tea.KeyEnter:
		if len(state.Options) > 0 {
			apply(strings.TrimSpace(state.Options[state.Index]))
		}
		m.submode = SubmodeNone
		return *m, nil
	}

	switch msg.String() {
	case "up", "k":
		if len(state.Options) > 0 {
			if state.Index > 0 {
				state.Index--
			} else {
				state.Index = len(state.Options) - 1
			}
		}
		return *m, nil
	case "down", "j":
		if len(state.Options) > 0 {
			state.Index = (state.Index + 1) % len(state.Options)
		}
		return *m, nil
	default:
		return *m, nil
	}
}

func (m *Model) panMain(delta int) {
	pane := m.activePane
	m.mainColScroll[pane] = clamp(m.mainColScroll[pane]+delta, 0, m.maxMainColScroll())
}

func (m *Model) applyMainFilter() {
	if m.mode == ModeSelector && m.sourceTrees != nil {
		cacheKey := strings.ToLower(strings.TrimSpace(m.filterQuery))
		if cacheKey == "" {
			m.rowsByPane[0] = m.allRowsByPane[0]
			m.rowsByPane[1] = m.allRowsByPane[1]
		} else if cached, ok := m.filterCache[cacheKey]; ok {
			m.rowsByPane[0] = cached.rows0
			m.rowsByPane[1] = cached.rows1
		} else {
			matcher := buildQueryMatcher(cacheKey)
			filteredTrees := filterResponseTrees(m.sourceTrees, matcher)
			rows := flattenTrees(filteredTrees)
			m.rowsByPane[0] = rows
			m.rowsByPane[1] = singleRows(rows)
			m.filterCache[cacheKey] = filteredRowsCache{rows0: m.rowsByPane[0], rows1: m.rowsByPane[1]}
		}

		m.normalizeCursor(0)
		m.normalizeCursor(1)
		m.mainColScroll[0] = clamp(m.mainColScroll[0], 0, m.maxMainColScrollForPane(0))
		m.mainColScroll[1] = clamp(m.mainColScroll[1], 0, m.maxMainColScrollForPane(1))
		m.pruneInvisibleSelections()
		return
	}

	matcher := queryMatcher{}
	if strings.TrimSpace(m.filterQuery) != "" {
		matcher = buildQueryMatcher(m.filterQuery)
	}

	for pane := range m.rowsByPane {
		source := m.allRowsByPane[pane]
		if strings.TrimSpace(m.filterQuery) == "" {
			m.rowsByPane[pane] = source
			m.normalizeCursor(pane)
			m.mainColScroll[pane] = clamp(m.mainColScroll[pane], 0, m.maxMainColScrollForPane(pane))
			continue
		}

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
		m.mainColScroll[pane] = clamp(m.mainColScroll[pane], 0, m.maxMainColScrollForPane(pane))
	}

	m.pruneInvisibleSelections()
}

func (m *Model) pruneInvisibleSelections() {
	visibleByPane := [2]map[string]struct{}{}
	for pane := range m.rowsByPane {
		visible := make(map[string]struct{}, len(m.rowsByPane[pane]))
		for _, row := range m.rowsByPane[pane] {
			if row.Separator {
				continue
			}
			visible[row.Key] = struct{}{}
		}
		visibleByPane[pane] = visible
	}
	for pane := range m.selected {
		for key := range m.selected[pane] {
			if _, ok := visibleByPane[pane][key]; !ok {
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
