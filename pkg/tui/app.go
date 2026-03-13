package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	kube "github.com/karloie/kompass/pkg/kube"
	"sigs.k8s.io/yaml"
)

type Mode int

const (
	ModeSelector Mode = iota
	ModeServerDashboard
)

type Options struct {
	Mode       Mode
	Trees      *kube.Trees
	Context    string
	Namespace  string
	OutputJSON bool
	Plain      bool
}

type row struct {
	Key      string
	Type     string
	Name     string
	Status   string
	Metadata map[string]any
	Depth    int
}

type modalKind string

const (
	modalYAML modalKind = "yaml"
	modalHelp modalKind = "help"
)

type viewerModal struct {
	Kind      modalKind
	Title     string
	Lines     []string
	Raw       string
	Scroll    int
	ColScroll int

	SearchMode   bool
	SearchQuery  string
	MatchLines   []int
	ActiveMatch  int
	ActionStatus string
}

type editorDoneMsg struct {
	err error
}

type model struct {
	mode      Mode
	context   string
	namespace string

	width  int
	height int

	activePane   int
	activeColumn int

	rowsByPane   [2][]row
	cursorByPane [2]int
	selected     [2]map[string]bool
	resources    map[string]*kube.Resource

	footerHeight  int
	modal         *viewerModal
	emitSelection bool
}

var (
	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24")).Padding(0, 1)
	footerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("238")).Padding(0, 1)
	focusedCell       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("31"))
	matchLineStyle    = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	activeMatchStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("166"))
	lineNumberStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	gutterMatchStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	gutterActiveStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("166"))
	termMatchStyle    = lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("227"))
	termActiveStyle   = lipgloss.NewStyle().Underline(true).Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("166"))
	selectedMarker    = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("[x]")
	unselectedMarker  = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("[ ]")
)

func Run(opts Options) error {
	m := newModel(opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if opts.Mode != ModeSelector {
		return nil
	}
	fm, ok := finalModel.(model)
	if !ok || !fm.emitSelection {
		return nil
	}
	output, err := formatSelectionOutput(fm.keysForOutput(), opts.OutputJSON)
	if err != nil {
		return err
	}
	if output != "" {
		fmt.Print(output)
	}
	return nil
}

func formatSelectionOutput(keys []string, outputJSON bool) (string, error) {
	if len(keys) == 0 {
		return "", nil
	}
	if outputJSON {
		b, err := json.Marshal(keys)
		if err != nil {
			return "", err
		}
		return string(b) + "\n", nil
	}
	return strings.Join(keys, "\n") + "\n", nil
}

func newModel(opts Options) model {
	m := model{
		mode:         opts.Mode,
		context:      opts.Context,
		namespace:    opts.Namespace,
		footerHeight: 1,
	}
	m.selected[0] = map[string]bool{}
	m.selected[1] = map[string]bool{}
	m.resources = map[string]*kube.Resource{}

	if opts.Mode == ModeSelector && opts.Trees != nil {
		allRows := flattenTrees(opts.Trees)
		m.rowsByPane[0] = allRows
		m.rowsByPane[1] = singleRows(allRows)
		for k, v := range opts.Trees.Nodes {
			m.resources[k] = v
		}
	}

	if opts.Mode == ModeServerDashboard {
		m.rowsByPane[0] = []row{
			{Key: "todo/1", Type: "todo", Name: "Review recent requests", Status: "new", Metadata: map[string]any{"owner": "you"}},
			{Key: "todo/2", Type: "todo", Name: "Investigate cache misses", Status: "in-progress", Metadata: map[string]any{"area": "cache"}},
			{Key: "todo/3", Type: "todo", Name: "Verify release artifacts", Status: "new", Metadata: map[string]any{"priority": "high"}},
		}
	}

	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if m.modal != nil {
			return m.handleModalKey(msg)
		}

		return m.handleMainKey(msg)
	}

	switch msg := msg.(type) {
	case editorDoneMsg:
		if m.modal != nil {
			if msg.err != nil {
				m.modal.ActionStatus = "editor failed: " + msg.err.Error()
			} else {
				m.modal.ActionStatus = "editor closed"
			}
		}
	}
	return m, nil
}

func (m *model) handleModalSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return *m, tea.Quit
	case tea.KeyCtrlU:
		m.modal.SearchQuery = ""
	case tea.KeyEsc:
		m.modal.SearchMode = false
	case tea.KeyEnter:
		m.modal.SearchMode = false
		m.applyModalSearch()
	case tea.KeyBackspace:
		if len(m.modal.SearchQuery) > 0 {
			r := []rune(m.modal.SearchQuery)
			m.modal.SearchQuery = string(r[:len(r)-1])
		}
	default:
		if len(msg.Runes) > 0 {
			m.modal.SearchQuery += string(msg.Runes)
		}
	}
	return *m, nil
}

func (m *model) handleModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modal.SearchMode {
		return m.handleModalSearchKey(msg)
	}

	switch msg.String() {
	case "ctrl+c":
		return *m, tea.Quit
	case "esc", "q":
		m.modal = nil
	case "left", "h":
		m.modal.ColScroll = maxInt(0, m.modal.ColScroll-4)
	case "right", "l":
		m.modal.ColScroll = minInt(m.modalMaxColScroll(), m.modal.ColScroll+4)
	case "home":
		m.modal.ColScroll = 0
	case "end":
		m.modal.ColScroll = m.modalMaxColScroll()
	case "g":
		m.modal.Scroll = 0
	case "G":
		m.modal.Scroll = maxInt(0, len(m.modal.Lines)-1)
	case "up", "k":
		if m.modal.Scroll > 0 {
			m.modal.Scroll--
		}
	case "down", "j":
		if m.modal.Scroll < maxInt(0, len(m.modal.Lines)-1) {
			m.modal.Scroll++
		}
	case "pgup":
		m.modal.Scroll = maxInt(0, m.modal.Scroll-10)
	case "pgdown":
		m.modal.Scroll = minInt(maxInt(0, len(m.modal.Lines)-1), m.modal.Scroll+10)
	case "/":
		// Preserve last query for quick repeated filtering.
		m.modal.SearchMode = true
		m.modal.ActionStatus = ""
	case "n":
		m.jumpModalMatch(1)
	case "N":
		m.jumpModalMatch(-1)
	case "y":
		if err := copyToClipboard(m.modal.Raw); err != nil {
			m.modal.ActionStatus = "copy failed: " + err.Error()
		} else {
			m.modal.ActionStatus = "copied to clipboard"
		}
	case "e":
		return *m, openInEditorCmd(m.modal.Raw)
	case "o":
		m.emitSelection = true
		return *m, tea.Quit
	}
	return *m, nil
}

func (m *model) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			m.activePane = m.nextAvailablePane(1)
		}
	case "shift+tab":
		if m.mode == ModeSelector {
			m.activePane = m.nextAvailablePane(-1)
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
	case "left":
		m.activeColumn = (m.activeColumn + 3) % 4
	case "right":
		m.activeColumn = (m.activeColumn + 1) % 4
	case "up", "k":
		if m.cursorByPane[m.activePane] > 0 {
			m.cursorByPane[m.activePane]--
		}
	case "down", "j":
		if m.cursorByPane[m.activePane] < len(m.rowsByPane[m.activePane])-1 {
			m.cursorByPane[m.activePane]++
		}
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
			m.modal = openYAMLModal(*r, m.resources[r.Key])
		}
	case "v":
		m.modal = openSelectionModal(m.keysForOutput())
	case "o":
		m.emitSelection = true
		return *m, tea.Quit
	case "?":
		m.modal = openHelpModal()
	}
	return *m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}

	if m.modal != nil {
		return m.viewModal()
	}

	header := m.viewHeader()
	footer := m.viewFooter()
	bodyHeight := maxInt(1, m.height-1-m.footerHeight)
	body := m.viewRows(bodyHeight)

	return strings.Join([]string{header, body, footer}, "\n")
}

func (m model) viewHeader() string {
	paneName := "Selector"
	if m.mode == ModeServerDashboard {
		paneName = "Dashboard"
	} else if m.activePane == 1 {
		paneName = "Single"
	}
	columns := []string{"type", "name", "status", "metadata"}
	selectedCount := len(m.selected[m.activePane])
	text := fmt.Sprintf("%s | col:%s | selected:%d | Tab pane | arrows nav | Space select | Enter inspect | v view selected | o output keys | ? help | Esc quit", paneName, columns[m.activeColumn], selectedCount)
	return fit(headerStyle.Render(text), m.width)
}

func (m model) viewFooter() string {
	r := m.currentRow()
	if r == nil {
		return fit(footerStyle.Render("No items"), m.width)
	}
	text := footerSummary(m.context, m.namespace, r, m.activeColumn)
	line := fit(footerStyle.Render(text), m.width)
	if m.footerHeight == 1 {
		return line
	}

	lines := []string{line}
	for i := 1; i < m.footerHeight; i++ {
		lines = append(lines, fit(footerStyle.Render(" "), m.width))
	}
	return strings.Join(lines, "\n")
}

func footerSummary(context, namespace string, r *row, activeColumn int) string {
	if r == nil {
		return "No items"
	}

	base := fmt.Sprintf("%s/%s | %s %s [%s]", context, namespace, r.Type, r.Name, r.Status)
	if len(r.Metadata) == 0 {
		return base
	}

	switch activeColumn {
	case 0: // type
		return fmt.Sprintf("%s type=%s depth=%d", base, r.Type, r.Depth)
	case 1: // name
		return fmt.Sprintf("%s key=%s", base, r.Key)
	case 2: // status
		reason, _ := r.Metadata["reason"].(string)
		if reason != "" {
			return fmt.Sprintf("%s reason=%s", base, reason)
		}
		return base
	default: // metadata
		meta := summarizeMetadata(r.Metadata)
		if meta == "" {
			return base
		}
		return fmt.Sprintf("%s %s", base, meta)
	}
}

func (m model) viewRows(height int) string {
	rows := m.rowsByPane[m.activePane]
	if len(rows) == 0 {
		return strings.Repeat("\n", maxInt(0, height-1)) + "No items"
	}

	cursor := clamp(m.cursorByPane[m.activePane], 0, len(rows)-1)
	start := maxInt(0, cursor-height/2)
	end := minInt(len(rows), start+height)
	if end-start < height {
		start = maxInt(0, end-height)
	}

	lines := make([]string, 0, height)
	for i := start; i < end; i++ {
		lines = append(lines, m.renderRow(rows[i], i == cursor))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m model) renderRow(r row, focused bool) string {
	marker := unselectedMarker
	if m.selected[m.activePane][r.Key] {
		marker = selectedMarker
	}

	indent := strings.Repeat("  ", maxInt(0, r.Depth))
	typeCol := pad(r.Type, 16)
	nameCol := pad(indent+r.Name, maxInt(18, m.width/3))
	statusCol := pad(r.Status, 14)
	metaCol := truncate(summarizeMetadata(r.Metadata), maxInt(10, m.width-40))

	cols := []string{typeCol, nameCol, statusCol, metaCol}
	if focused {
		cols[m.activeColumn] = focusedCell.Render(cols[m.activeColumn])
	}

	line := fmt.Sprintf("%s %s | %s | %s | %s", marker, cols[0], cols[1], cols[2], cols[3])
	return fit(line, m.width)
}

func (m model) viewModal() string {
	headerText := m.modalHeaderText()
	footerText := m.modalFooterText()
	header := fit(headerStyle.Render(headerText), m.width)
	footer := fit(footerStyle.Render(footerText), m.width)
	bodyHeight := maxInt(1, m.height-2)
	bodyLines := m.modalBodyLines(bodyHeight)
	for len(bodyLines) < bodyHeight {
		bodyLines = append(bodyLines, "")
	}
	return strings.Join([]string{header, strings.Join(bodyLines, "\n"), footer}, "\n")
}

func (m model) modalHeaderText() string {
	headerText := fmt.Sprintf("YAML | %s | Esc close", m.modal.Title)
	if m.modal.Kind == modalHelp {
		return "HELP | Keybindings | Esc close"
	}
	if m.modal.SearchMode {
		headerText = "YAML | SEARCH | Enter apply | Esc cancel"
	}
	if m.modal.Kind != modalYAML {
		return headerText
	}

	lineInfo := fmt.Sprintf("line %d/%d col %d", minInt(len(m.modal.Lines), m.modal.Scroll+1), len(m.modal.Lines), m.modal.ColScroll+1)
	if len(m.modal.MatchLines) > 0 {
		lineInfo = fmt.Sprintf("%s | match %d/%d", lineInfo, m.modal.ActiveMatch+1, len(m.modal.MatchLines))
	}
	return fmt.Sprintf("%s | %s", headerText, lineInfo)
}

func (m model) modalFooterText() string {
	footerText := "Up/Down scroll | PgUp/PgDn page | g/G top/bottom | Left/Right pan | Home/End line start/end | / search | n/N next/prev | y copy | e edit | Esc close"
	if m.modal.Kind == modalHelp {
		footerText = "Tab/Shift+Tab panes | arrows nav | Space select | Enter inspect | Esc close"
	} else if m.modal.SearchMode {
		footerText = "Search: " + m.modal.SearchQuery
	} else if len(m.modal.MatchLines) > 0 {
		footerText = fmt.Sprintf("%s | match %d/%d", footerText, m.modal.ActiveMatch+1, len(m.modal.MatchLines))
	}
	if m.modal.ActionStatus != "" {
		footerText += " | " + m.modal.ActionStatus
	}
	return footerText
}

func (m model) modalBodyLines(bodyHeight int) []string {
	start := clamp(m.modal.Scroll, 0, maxInt(0, len(m.modal.Lines)-1))
	end := minInt(len(m.modal.Lines), start+bodyHeight)
	bodyLines := make([]string, 0, bodyHeight)
	lineNumberWidth := len(fmt.Sprintf("%d", maxInt(1, len(m.modal.Lines))))
	contentWidth := maxInt(1, m.width-lineNumberWidth-4)
	activeMatchLine := m.modal.activeMatchLine()

	for i := start; i < end; i++ {
		line := visibleSegment(m.modal.Lines[i], m.modal.ColScroll, contentWidth)
		line = highlightSearchTerm(line, m.modal.SearchQuery, i == activeMatchLine)
		line = styleModalLine(line, i, m.modal.MatchLines, activeMatchLine)
		prefix := modalLinePrefix(i, lineNumberWidth, m.modal.MatchLines, activeMatchLine)
		bodyLines = append(bodyLines, prefix+line)
	}
	return bodyLines
}

func (m model) currentRow() *row {
	rows := m.rowsByPane[m.activePane]
	if len(rows) == 0 {
		return nil
	}
	idx := clamp(m.cursorByPane[m.activePane], 0, len(rows)-1)
	return &rows[idx]
}

func (m *model) clearActiveSelection() bool {
	if len(m.selected[m.activePane]) == 0 {
		return false
	}
	m.selected[m.activePane] = map[string]bool{}
	return true
}

func (m model) paneAvailable(pane int) bool {
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

func (m model) nextAvailablePane(direction int) int {
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

func flattenTrees(trees *kube.Trees) []row {
	rows := make([]row, 0, 128)
	for _, root := range trees.Trees {
		flattenNode(&rows, root, 0)
	}
	return rows
}

func flattenNode(rows *[]row, n *kube.Tree, depth int) {
	if n == nil {
		return
	}
	meta := map[string]any{}
	for k, v := range n.Meta {
		meta[k] = v
	}
	name := stringMeta(meta, "name", n.Key)
	status := stringMeta(meta, "status", "")
	*rows = append(*rows, row{Key: n.Key, Type: n.Type, Name: name, Status: status, Metadata: meta, Depth: depth})
	for _, c := range n.Children {
		flattenNode(rows, c, depth+1)
	}
}

func singleRows(rows []row) []row {
	out := make([]row, 0)
	for _, r := range rows {
		if isSingle, ok := r.Metadata["orphaned"].(bool); ok && isSingle {
			out = append(out, r)
		}
	}
	return out
}

func openYAMLModal(r row, resource *kube.Resource) *viewerModal {
	var content any
	if resource != nil && resource.Resource != nil {
		content = resource.Resource
	} else {
		content = map[string]any{
			"key":      r.Key,
			"type":     r.Type,
			"name":     r.Name,
			"status":   r.Status,
			"metadata": r.Metadata,
		}
	}
	b, err := yaml.Marshal(content)
	if err != nil {
		b = []byte("error: failed to render yaml")
	}
	raw := strings.TrimRight(string(b), "\n")
	lines := strings.Split(raw, "\n")
	return &viewerModal{Kind: modalYAML, Title: fmt.Sprintf("%s/%s", r.Type, r.Name), Lines: lines, Raw: raw}
}

func openHelpModal() *viewerModal {
	lines := []string{
		"Navigation",
		"  Up/Down or j/k  move row",
		"  Left/Right      switch focused column (type/name/status/metadata)",
		"  Tab             next pane",
		"  Shift+Tab       previous pane",
		"  1/2             jump to Selector/Single",
		"",
		"Actions",
		"  Space           toggle row selection",
		"  Enter           open YAML modal for current row",
		"  v               open selected rows modal",
		"  o               output selected/current keys and quit",
		"  + / -           increase/decrease footer panel height",
		"",
		"Modal",
		"  Up/Down, PgUp/PgDn  scroll",
		"  g / G               jump to top/bottom",
		"  Left/Right or h/l   pan long lines",
		"  Home/End            pan to line start/end",
		"  /                    start search",
		"  Ctrl+U               clear search query",
		"  n / N                next/previous match",
		"  y                    copy modal content",
		"  e                    open in $EDITOR (read-only where supported)",
		"  Esc or q            close modal",
		"",
		"Exit",
		"  Esc / Ctrl+C     quit application",
	}
	raw := strings.Join(lines, "\n")
	return &viewerModal{Kind: modalHelp, Title: "Keybindings", Lines: lines, Raw: raw}
}

func openSelectionModal(keys []string) *viewerModal {
	if len(keys) == 0 {
		keys = []string{"(no rows)"}
	}
	content := map[string]any{
		"selectedCount": len(keys),
		"selectedKeys":  keys,
	}
	b, err := yaml.Marshal(content)
	if err != nil {
		b = []byte("error: failed to render selected rows")
	}
	raw := strings.TrimRight(string(b), "\n")
	lines := strings.Split(raw, "\n")
	return &viewerModal{Kind: modalYAML, Title: "selected/rows", Lines: lines, Raw: raw}
}

func (m model) keysForOutput() []string {
	keysSet := map[string]bool{}
	for pane := range m.selected {
		for key := range m.selected[pane] {
			keysSet[key] = true
		}
	}
	if len(keysSet) == 0 {
		if r := m.currentRow(); r != nil {
			keysSet[r.Key] = true
		}
	}
	keys := make([]string, 0, len(keysSet))
	for key := range keysSet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (m *model) applyModalSearch() {
	if m.modal == nil || m.modal.Kind != modalYAML {
		return
	}
	query := strings.TrimSpace(strings.ToLower(m.modal.SearchQuery))
	if query == "" {
		m.modal.MatchLines = nil
		m.modal.ActiveMatch = 0
		m.modal.ActionStatus = ""
		return
	}

	matches := make([]int, 0)
	for i, line := range m.modal.Lines {
		if strings.Contains(strings.ToLower(line), query) {
			matches = append(matches, i)
		}
	}
	m.modal.MatchLines = matches
	m.modal.ActiveMatch = 0
	if len(matches) == 0 {
		m.modal.ActionStatus = "no matches"
		return
	}
	m.modal.Scroll = matches[0]
	m.ensureActiveMatchVisible()
	m.modal.ActionStatus = fmt.Sprintf("found %d matches", len(matches))
}

func (m *model) jumpModalMatch(direction int) {
	if m.modal == nil || len(m.modal.MatchLines) == 0 {
		if m.modal != nil {
			m.modal.ActionStatus = "no matches"
		}
		return
	}
	count := len(m.modal.MatchLines)
	idx := (m.modal.ActiveMatch + direction + count) % count
	m.modal.ActiveMatch = idx
	m.modal.Scroll = m.modal.MatchLines[idx]
	m.ensureActiveMatchVisible()
	m.modal.ActionStatus = ""
}

func (m *model) ensureActiveMatchVisible() {
	if m.modal == nil || len(m.modal.MatchLines) == 0 {
		return
	}
	activeLine := m.modal.activeMatchLine()
	if activeLine < 0 || activeLine >= len(m.modal.Lines) {
		return
	}

	query := strings.TrimSpace(m.modal.SearchQuery)
	if query == "" {
		return
	}

	matchCol := matchColumn(m.modal.Lines[activeLine], query)
	if matchCol < 0 {
		return
	}

	contentWidth := m.modalContentWidth()
	if contentWidth <= 0 {
		return
	}

	matchStart := matchCol
	matchEnd := matchCol + len([]rune(query)) - 1
	viewStart := m.modal.ColScroll
	viewEnd := viewStart + contentWidth - 1

	if matchStart < viewStart {
		m.modal.ColScroll = matchStart
	} else if matchEnd > viewEnd {
		m.modal.ColScroll = matchEnd - contentWidth + 1
	}
	m.modal.ColScroll = clamp(m.modal.ColScroll, 0, m.modalMaxColScroll())
}

func (m *model) modalMaxColScroll() int {
	if m.modal == nil || len(m.modal.Lines) == 0 {
		return 0
	}
	contentWidth := m.modalContentWidth()
	longest := 0
	for _, line := range m.modal.Lines {
		l := len([]rune(line))
		if l > longest {
			longest = l
		}
	}
	return maxInt(0, longest-contentWidth)
}

func (m *model) modalContentWidth() int {
	if m.modal == nil {
		return 1
	}
	lineNumberWidth := len(fmt.Sprintf("%d", maxInt(1, len(m.modal.Lines))))
	return maxInt(1, m.width-lineNumberWidth-4)
}

func styleModalLine(line string, lineIndex int, matchLines []int, activeMatchLine int) string {
	if lineIndex == activeMatchLine {
		return activeMatchStyle.Render(line)
	}
	if containsInt(matchLines, lineIndex) {
		return matchLineStyle.Render(line)
	}
	return line
}

func modalLinePrefix(lineIndex, lineNumberWidth int, matchLines []int, activeMatchLine int) string {
	marker := modalGutterMarker(lineIndex, matchLines, activeMatchLine)
	switch marker {
	case ">":
		marker = gutterActiveStyle.Render(marker)
	case "*":
		marker = gutterMatchStyle.Render(marker)
	}
	lineNumber := lineNumberStyle.Render(fmt.Sprintf("%*d ", lineNumberWidth, lineIndex+1))
	return marker + " " + lineNumber
}

func modalGutterMarker(lineIndex int, matchLines []int, activeMatchLine int) string {
	if lineIndex == activeMatchLine {
		return ">"
	}
	if containsInt(matchLines, lineIndex) {
		return "*"
	}
	return " "
}

func highlightSearchTerm(line, query string, active bool) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return line
	}

	lowerLine := strings.ToLower(line)
	lowerQuery := strings.ToLower(q)
	if !strings.Contains(lowerLine, lowerQuery) {
		return line
	}

	style := termMatchStyle
	if active {
		style = termActiveStyle
	}

	result := strings.Builder{}
	start := 0
	for {
		idx := strings.Index(strings.ToLower(line[start:]), lowerQuery)
		if idx < 0 {
			result.WriteString(line[start:])
			break
		}
		idx += start
		result.WriteString(line[start:idx])
		end := idx + len(q)
		if end > len(line) {
			end = len(line)
		}
		result.WriteString(style.Render(line[idx:end]))
		start = end
	}

	return result.String()
}

func visibleSegment(line string, colScroll, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(line)
	if len(r) == 0 {
		return ""
	}

	start := clamp(colScroll, 0, len(r))
	if start >= len(r) {
		return ""
	}
	end := minInt(len(r), start+width)
	out := string(r[start:end])
	if end < len(r) && width > 1 {
		out = truncate(out, width-1) + "~"
	}
	return out
}

func matchColumn(line, query string) int {
	q := strings.TrimSpace(query)
	if q == "" {
		return -1
	}

	lineRunes := []rune(line)
	lowerLineRunes := []rune(strings.ToLower(line))
	lowerQueryRunes := []rune(strings.ToLower(q))
	if len(lowerQueryRunes) == 0 || len(lowerLineRunes) < len(lowerQueryRunes) {
		return -1
	}

	for i := 0; i <= len(lowerLineRunes)-len(lowerQueryRunes); i++ {
		matched := true
		for j := 0; j < len(lowerQueryRunes); j++ {
			if lowerLineRunes[i+j] != lowerQueryRunes[j] {
				matched = false
				break
			}
		}
		if matched {
			return len(lineRunes[:i])
		}
	}

	return -1
}

func containsInt(values []int, needle int) bool {
	for _, v := range values {
		if v == needle {
			return true
		}
	}
	return false
}

func (m *viewerModal) activeMatchLine() int {
	if m == nil || len(m.MatchLines) == 0 {
		return -1
	}
	if m.ActiveMatch < 0 || m.ActiveMatch >= len(m.MatchLines) {
		return -1
	}
	return m.MatchLines[m.ActiveMatch]
}

func copyToClipboard(content string) error {
	candidates := [][]string{{"wl-copy"}, {"xclip", "-selection", "clipboard"}, {"pbcopy"}}
	for _, c := range candidates {
		if _, err := exec.LookPath(c[0]); err != nil {
			continue
		}
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Stdin = strings.NewReader(content)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no supported clipboard tool found (wl-copy/xclip/pbcopy)")
}

func openInEditorCmd(content string) tea.Cmd {
	return func() tea.Msg {
		tmp, err := os.CreateTemp("", "kompass-yaml-*.yaml")
		if err != nil {
			return editorDoneMsg{err: err}
		}
		defer os.Remove(tmp.Name())

		if _, err := tmp.WriteString(content); err != nil {
			_ = tmp.Close()
			return editorDoneMsg{err: err}
		}
		_ = tmp.Close()

		editorCmd, editorArgs := resolveEditorCommand(os.Getenv("EDITOR"), tmp.Name())
		cmd := exec.Command(editorCmd, editorArgs...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return editorDoneMsg{err: cmd.Run()}
	}
}

func resolveEditorCommand(editorEnv, filePath string) (string, []string) {
	editor := strings.TrimSpace(editorEnv)
	if editor == "" {
		editor = "vi"
	}

	parts := strings.Fields(editor)
	bin := parts[0]
	args := append([]string{}, parts[1:]...)
	base := strings.ToLower(filepath.Base(bin))

	// Open known editors in read-only/viewer mode to avoid accidental mutations.
	switch base {
	case "vi", "vim", "nvim", "view", "vim.basic", "gvim":
		if !hasArg(args, "-R") {
			args = append(args, "-R")
		}
	case "nano", "pico":
		if !hasArg(args, "-v") {
			args = append(args, "-v")
		}
	case "code", "code-insiders", "cursor", "windsurf":
		if !hasArg(args, "--wait") {
			args = append(args, "--wait")
		}
		if !hasArg(args, "--readonly") {
			args = append(args, "--readonly")
		}
	}

	args = append(args, filePath)
	return bin, args
}

func hasArg(args []string, needle string) bool {
	for _, arg := range args {
		if arg == needle {
			return true
		}
	}
	return false
}

func stringMeta(meta map[string]any, key, fallback string) string {
	if v, ok := meta[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

func summarizeMetadata(meta map[string]any) string {
	if len(meta) == 0 {
		return ""
	}
	keys := make([]string, 0, len(meta))
	for k := range meta {
		if k == "name" || k == "status" || k == "orphaned" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	parts := make([]string, 0, minInt(3, len(keys)))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, meta[k]))
		if len(parts) == 3 {
			break
		}
	}
	return strings.Join(parts, " ")
}

func fit(s string, width int) string {
	if width <= 0 {
		return s
	}
	return truncate(s, width)
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return string(r[:1])
	}
	if width <= 3 {
		return string(r[:width])
	}
	return string(r[:width-3]) + "..."
}

func pad(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) >= width {
		return string(r[:width])
	}
	return s + strings.Repeat(" ", width-len(r))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
