package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	kube "github.com/karloie/kompass/pkg/kube"
)

type fileKind string

const (
	fileYAML fileKind = "yaml"
	fileHelp fileKind = "help"
)

const (
	FileYAML = fileYAML
	FileHelp = fileHelp
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

// Backward-compatible type names used across tests and split files.
type ViewFile = viewerFile

type FileModel struct {
	mode      Mode
	context   string
	namespace string

	width  int
	height int

	activePane   int
	activeColumn int

	rowsByPane   [2][]Row
	cursorByPane [2]int
	selected     [2]map[string]bool
	resources    map[string]*kube.Resource

	footerHeight  int
	file          *viewerFile
	emitSelection bool
	lastNavDir    int
	navRepeat     int
	navLastAt     time.Time
	now           func() time.Time
}

func (m FileModel) Init() tea.Cmd {
	return nil
}

func (m *FileModel) handleFileKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m.jumpMatch(1)
	case "N":
		m.jumpMatch(-1)
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

func (m *FileModel) handleFileSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return *m, tea.Quit
	case tea.KeyCtrlU:
		m.file.SearchQuery = ""
	case tea.KeyEsc:
		m.file.SearchMode = false
	case tea.KeyEnter:
		m.file.SearchMode = false
		m.applySearch()
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

func (m FileModel) viewFile() string {
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

func (m FileModel) fileHeaderText() string {
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

func (m FileModel) fileFooterText() string {
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

func (m FileModel) fileRowsLines(rowsHeight int) []string {
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
