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
	tree "github.com/karloie/kompass/pkg/tree"
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
	Key       string
	Type      string
	Name      string
	Line      string
	PlainLine string
	Status    string
	Metadata  map[string]any
	Depth     int
}

type fileKind string

const (
	fileYAML fileKind = "yaml"
	fileHelp fileKind = "help"
)

type viewerFile struct {
	Kind      fileKind
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
	file          *viewerFile
	emitSelection bool
}

var (
	headerStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24")).Padding(0, 1)
	footerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("238")).Padding(0, 1)
	fileedCell        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("31"))
	selectedRowStyle  = lipgloss.NewStyle().Bold(true)
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
		if m.file != nil {
			return m.handleFileKey(msg)
		}

		return m.handleMainKey(msg)
	}

	switch msg := msg.(type) {
	case editorDoneMsg:
		if m.file != nil {
			if msg.err != nil {
				m.file.ActionStatus = "editor failed: " + msg.err.Error()
			} else {
				m.file.ActionStatus = "editor closed"
			}
		}
	}
	return m, nil
}

func (m *model) handleFileSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return *m, tea.Quit
	case tea.KeyCtrlU:
		m.file.SearchQuery = ""
	case tea.KeyEsc:
		m.file.SearchMode = false
	case tea.KeyEnter:
		m.file.SearchMode = false
		m.applyFileSearch()
	case tea.KeyBackspace:
		if len(m.file.SearchQuery) > 0 {
			r := []rune(m.file.SearchQuery)
			m.file.SearchQuery = string(r[:len(r)-1])
		}
	default:
		if len(msg.Runes) > 0 {
			m.file.SearchQuery += string(msg.Runes)
		}
	}
	return *m, nil
}

func (m *model) handleFileKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.file.SearchMode {
		return m.handleFileSearchKey(msg)
	}

	switch msg.String() {
	case "ctrl+c":
		return *m, tea.Quit
	case "esc", "q":
		m.file = nil
	case "enter":
		m.file = nil
	case "left", "h":
		m.file.ColScroll = maxInt(0, m.file.ColScroll-4)
	case "right", "l":
		m.file.ColScroll = minInt(m.fileMaxColScroll(), m.file.ColScroll+4)
	case "home":
		m.file.ColScroll = 0
	case "end":
		m.file.ColScroll = m.fileMaxColScroll()
	case "g":
		m.file.Scroll = 0
	case "G":
		m.file.Scroll = maxInt(0, len(m.file.Lines)-1)
	case "up", "k":
		if m.file.Scroll > 0 {
			m.file.Scroll--
		}
	case "down", "j":
		if m.file.Scroll < maxInt(0, len(m.file.Lines)-1) {
			m.file.Scroll++
		}
	case "pgup":
		m.file.Scroll = maxInt(0, m.file.Scroll-10)
	case "pgdown":
		m.file.Scroll = minInt(maxInt(0, len(m.file.Lines)-1), m.file.Scroll+10)
	case "/":
		// Preserve last query for quick repeated filtering.
		m.file.SearchMode = true
		m.file.ActionStatus = ""
	case "n":
		m.jumpFileMatch(1)
	case "N":
		m.jumpFileMatch(-1)
	case "y":
		if err := copyToClipboard(m.file.Raw); err != nil {
			m.file.ActionStatus = "copy failed: " + err.Error()
		} else {
			m.file.ActionStatus = "copied to clipboard"
		}
	case "e":
		return *m, openInEditorCmd(m.file.Raw)
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
			m.file = openYAMLFile(*r, m.resources[r.Key])
		}
	case "v":
		m.file = openSelectionFile(m.keysForOutput())
	case "o":
		m.emitSelection = true
		return *m, tea.Quit
	case "?":
		m.file = openHelpFile()
	}
	return *m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}

	if m.file != nil {
		return m.viewFile()
	}

	header := m.viewHeader()
	footer := m.viewFooter()
	rowsHeight := maxInt(1, m.height-1-m.footerHeight)
	rows := m.viewRows(rowsHeight)

	return strings.Join([]string{header, rows, footer}, "\n")
}

func (m model) viewHeader() string {
	paneName := "Selector"
	if m.mode == ModeServerDashboard {
		paneName = "Dashboard"
	} else if m.activePane == 1 {
		paneName = "Single"
	}
	selectedCount := len(m.selected[m.activePane])
	text := fmt.Sprintf("%s | selected:%d | Tab pane | Up/Down rows | Space select | Enter inspect | v view selected | o output keys | ? help | Esc quit", paneName, selectedCount)
	return renderFullWidthBar(headerStyle, text, m.width)
}

func renderFullWidthBar(style lipgloss.Style, text string, width int) string {
	if width <= 0 {
		return style.Render(text)
	}
	contentWidth := maxInt(0, width-2)
	content := pad(truncate(text, contentWidth), contentWidth)
	return style.Render(content)
}

func (m model) viewFooter() string {
	r := m.currentRow()
	if r == nil {
		return renderFullWidthBar(footerStyle, "No items", m.width)
	}
	text := footerSummary(m.context, m.namespace, r)
	line := renderFullWidthBar(footerStyle, text, m.width)
	if m.footerHeight == 1 {
		return line
	}

	lines := []string{line}
	for i := 1; i < m.footerHeight; i++ {
		lines = append(lines, renderFullWidthBar(footerStyle, "", m.width))
	}
	return strings.Join(lines, "\n")
}

func footerSummary(context, namespace string, r *row) string {
	if r == nil {
		return "No items"
	}

	base := fmt.Sprintf("%s/%s | %s %s [%s]", context, namespace, r.Type, r.Name, r.Status)
	return fmt.Sprintf("%s key=%s", base, r.Key)
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

func (m model) renderRow(r row, fileed bool) string {
	isSelected := m.selected[m.activePane][r.Key]
	marker := "[ ]"
	if isSelected {
		marker = "[x]"
	}
	if !fileed {
		if isSelected {
			marker = selectedMarker
		} else {
			marker = unselectedMarker
		}
	}

	lineContent := r.Line
	if fileed {
		lineContent = r.PlainLine
	}
	if strings.TrimSpace(lineContent) == "" {
		lineContent = r.Name
	}
	content := withSelectionMarker(lineContent, marker)
	if fileed {
		content = pad(truncate(content, maxInt(1, m.width)), maxInt(1, m.width))
		content = fileedCell.Render(content)
	} else if isSelected {
		content = selectedRowStyle.Render(content)
	}
	return content
}

func withSelectionMarker(line, marker string) string {
	for _, branch := range []string{"├─ ", "└─ "} {
		if idx := strings.Index(line, branch); idx >= 0 {
			insertPos := idx + len(branch)
			return line[:insertPos] + marker + " " + line[insertPos:]
		}
	}
	return marker + " " + line
}

func (m model) viewFile() string {
	headerText := m.fileHeaderText()
	footerText := m.fileFooterText()
	header := fit(headerStyle.Render(headerText), m.width)
	footer := fit(footerStyle.Render(footerText), m.width)
	rowsHeight := maxInt(1, m.height-2)
	rowsLines := m.fileRowsLines(rowsHeight)
	for len(rowsLines) < rowsHeight {
		rowsLines = append(rowsLines, "")
	}
	return strings.Join([]string{header, strings.Join(rowsLines, "\n"), footer}, "\n")
}

func (m model) fileHeaderText() string {
	headerText := fmt.Sprintf("FILE | %s | Esc close", m.file.Title)
	if m.file.Kind == fileHelp {
		return "HELP | Keybindings | Esc close"
	}
	if m.file.SearchMode {
		headerText = "FILE | SEARCH | Enter apply | Esc cancel"
	}
	if m.file.Kind != fileYAML {
		return headerText
	}

	lineInfo := fmt.Sprintf("line %d/%d col %d", minInt(len(m.file.Lines), m.file.Scroll+1), len(m.file.Lines), m.file.ColScroll+1)
	if len(m.file.MatchLines) > 0 {
		lineInfo = fmt.Sprintf("%s | match %d/%d", lineInfo, m.file.ActiveMatch+1, len(m.file.MatchLines))
	}
	return fmt.Sprintf("%s | %s", headerText, lineInfo)
}

func (m model) fileFooterText() string {
	footerText := "Up/Down scroll | PgUp/PgDn page | g/G top/bottom | Left/Right pan | Home/End line start/end | / search | n/N next/prev | y copy | e edit | Esc close"
	if m.file.Kind == fileHelp {
		footerText = "Tab/Shift+Tab panes | arrows rows | Space select | Enter inspect | Esc close"
	} else if m.file.SearchMode {
		footerText = "Search: " + m.file.SearchQuery
	} else if len(m.file.MatchLines) > 0 {
		footerText = fmt.Sprintf("%s | match %d/%d", footerText, m.file.ActiveMatch+1, len(m.file.MatchLines))
	}
	if m.file.ActionStatus != "" {
		footerText += " | " + m.file.ActionStatus
	}
	return footerText
}

func (m model) fileRowsLines(rowsHeight int) []string {
	start := clamp(m.file.Scroll, 0, maxInt(0, len(m.file.Lines)-1))
	end := minInt(len(m.file.Lines), start+rowsHeight)
	rowsLines := make([]string, 0, rowsHeight)
	lineNumberWidth := len(fmt.Sprintf("%d", maxInt(1, len(m.file.Lines))))
	contentWidth := maxInt(1, m.width-lineNumberWidth-4)
	activeMatchLine := m.file.activeMatchLine()

	for i := start; i < end; i++ {
		line := visibleSegment(m.file.Lines[i], m.file.ColScroll, contentWidth)
		line = highlightSearchTerm(line, m.file.SearchQuery, i == activeMatchLine)
		line = styleFileLine(line, i, m.file.MatchLines, activeMatchLine)
		prefix := fileLinePrefix(i, lineNumberWidth, m.file.MatchLines, activeMatchLine)
		rowsLines = append(rowsLines, prefix+line)
	}
	return rowsLines
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
		coloredRendered := strings.TrimRight(tree.RenderTree(root, trees.Nodes, false), "\n")
		plainRendered := strings.TrimRight(tree.RenderTree(root, trees.Nodes, true), "\n")
		coloredLines := []string{}
		plainLines := []string{}
		if coloredRendered != "" {
			coloredLines = strings.Split(coloredRendered, "\n")
		}
		if plainRendered != "" {
			plainLines = strings.Split(plainRendered, "\n")
		}
		lineIndex := 0
		flattenNode(&rows, root, 0, coloredLines, plainLines, &lineIndex)
	}
	return rows
}

func flattenNode(rows *[]row, n *kube.Tree, depth int, coloredLines, plainLines []string, lineIndex *int) {
	if n == nil {
		return
	}
	meta := map[string]any{}
	for k, v := range n.Meta {
		meta[k] = v
	}
	name := stringMeta(meta, "name", n.Key)
	status := stringMeta(meta, "status", "")
	line := name
	plainLine := name
	if lineIndex != nil {
		if *lineIndex < len(coloredLines) {
			line = coloredLines[*lineIndex]
		}
		if *lineIndex < len(plainLines) {
			plainLine = plainLines[*lineIndex]
		}
		*lineIndex++
	}
	*rows = append(*rows, row{Key: n.Key, Type: n.Type, Name: name, Line: line, PlainLine: plainLine, Status: status, Metadata: meta, Depth: depth})
	for _, c := range n.Children {
		flattenNode(rows, c, depth+1, coloredLines, plainLines, lineIndex)
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

func openYAMLFile(r row, resource *kube.Resource) *viewerFile {
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
	return &viewerFile{Kind: fileYAML, Title: fmt.Sprintf("%s/%s", r.Type, r.Name), Lines: lines, Raw: raw}
}

func openHelpFile() *viewerFile {
	lines := []string{
		"Rows",
		"  Up/Down or j/k  move row",
		"  Tab             next pane",
		"  Shift+Tab       previous pane",
		"  1/2             jump to Selector/Single",
		"",
		"Actions",
		"  Space           toggle row selection",
		"  Enter           open YAML file for current row",
		"  v               open selected rows file",
		"  o               output selected/current keys and quit",
		"  + / -           increase/decrease footer panel height",
		"",
		"File",
		"  Up/Down, PgUp/PgDn  scroll",
		"  g / G               jump to top/bottom",
		"  Left/Right or h/l   pan long lines",
		"  Home/End            pan to line start/end",
		"  /                    start search",
		"  Ctrl+U               clear search query",
		"  n / N                next/previous match",
		"  y                    copy file content",
		"  e                    open in $EDITOR (read-only where supported)",
		"  Esc or q            close file",
		"",
		"Exit",
		"  Esc / Ctrl+C     quit application",
	}
	raw := strings.Join(lines, "\n")
	return &viewerFile{Kind: fileHelp, Title: "Keybindings", Lines: lines, Raw: raw}
}

func openSelectionFile(keys []string) *viewerFile {
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
	return &viewerFile{Kind: fileYAML, Title: "selected/rows", Lines: lines, Raw: raw}
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

func (m *model) applyFileSearch() {
	if m.file == nil || m.file.Kind != fileYAML {
		return
	}
	query := strings.TrimSpace(strings.ToLower(m.file.SearchQuery))
	if query == "" {
		m.file.MatchLines = nil
		m.file.ActiveMatch = 0
		m.file.ActionStatus = ""
		return
	}

	matches := make([]int, 0)
	for i, line := range m.file.Lines {
		if strings.Contains(strings.ToLower(line), query) {
			matches = append(matches, i)
		}
	}
	m.file.MatchLines = matches
	m.file.ActiveMatch = 0
	if len(matches) == 0 {
		m.file.ActionStatus = "no matches"
		return
	}
	m.file.Scroll = matches[0]
	m.ensureActiveMatchVisible()
	m.file.ActionStatus = fmt.Sprintf("found %d matches", len(matches))
}

func (m *model) jumpFileMatch(direction int) {
	if m.file == nil || len(m.file.MatchLines) == 0 {
		if m.file != nil {
			m.file.ActionStatus = "no matches"
		}
		return
	}
	count := len(m.file.MatchLines)
	idx := (m.file.ActiveMatch + direction + count) % count
	m.file.ActiveMatch = idx
	m.file.Scroll = m.file.MatchLines[idx]
	m.ensureActiveMatchVisible()
	m.file.ActionStatus = ""
}

func (m *model) ensureActiveMatchVisible() {
	if m.file == nil || len(m.file.MatchLines) == 0 {
		return
	}
	activeLine := m.file.activeMatchLine()
	if activeLine < 0 || activeLine >= len(m.file.Lines) {
		return
	}

	query := strings.TrimSpace(m.file.SearchQuery)
	if query == "" {
		return
	}

	matchCol := matchColumn(m.file.Lines[activeLine], query)
	if matchCol < 0 {
		return
	}

	contentWidth := m.fileContentWidth()
	if contentWidth <= 0 {
		return
	}

	matchStart := matchCol
	matchEnd := matchCol + len([]rune(query)) - 1
	viewStart := m.file.ColScroll
	viewEnd := viewStart + contentWidth - 1

	if matchStart < viewStart {
		m.file.ColScroll = matchStart
	} else if matchEnd > viewEnd {
		m.file.ColScroll = matchEnd - contentWidth + 1
	}
	m.file.ColScroll = clamp(m.file.ColScroll, 0, m.fileMaxColScroll())
}

func (m *model) fileMaxColScroll() int {
	if m.file == nil || len(m.file.Lines) == 0 {
		return 0
	}
	contentWidth := m.fileContentWidth()
	longest := 0
	for _, line := range m.file.Lines {
		l := len([]rune(line))
		if l > longest {
			longest = l
		}
	}
	return maxInt(0, longest-contentWidth)
}

func (m *model) fileContentWidth() int {
	if m.file == nil {
		return 1
	}
	lineNumberWidth := len(fmt.Sprintf("%d", maxInt(1, len(m.file.Lines))))
	return maxInt(1, m.width-lineNumberWidth-4)
}

func styleFileLine(line string, lineIndex int, matchLines []int, activeMatchLine int) string {
	if lineIndex == activeMatchLine {
		return activeMatchStyle.Render(line)
	}
	if containsInt(matchLines, lineIndex) {
		return matchLineStyle.Render(line)
	}
	return line
}

func fileLinePrefix(lineIndex, lineNumberWidth int, matchLines []int, activeMatchLine int) string {
	marker := fileGutterMarker(lineIndex, matchLines, activeMatchLine)
	switch marker {
	case ">":
		marker = gutterActiveStyle.Render(marker)
	case "*":
		marker = gutterMatchStyle.Render(marker)
	}
	lineNumber := lineNumberStyle.Render(fmt.Sprintf("%*d ", lineNumberWidth, lineIndex+1))
	return marker + " " + lineNumber
}

func fileGutterMarker(lineIndex int, matchLines []int, activeMatchLine int) string {
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

func (m *viewerFile) activeMatchLine() int {
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
