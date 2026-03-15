package tui

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}

	if m.view != nil {
		return m.toString()
	}

	header := m.Header()
	footer := m.Footer()
	rowsHeight := maxInt(1, m.height-1-m.footerHeight)
	parts := []string{header}
	if m.mode == ModeSelector {
		parts = append(parts, m.filterBar())
		rowsHeight = maxInt(1, rowsHeight-1)
	}
	rows := m.ViewRows(rowsHeight)
	parts = append(parts, rows, footer)
	return strings.Join(parts, "\n")
}

func (m Model) toString() string {
	headerText := m.headerText()
	footerText := m.footerText()
	header := fit(headerStyle.Render(headerText), m.width)
	footer := fit(footerStyle.Render(footerText), m.width)
	rowsHeight := maxInt(1, m.height-2)
	rows := m.renderRows(rowsHeight)
	for len(rows) < rowsHeight {
		rows = append(rows, "")
	}
	return strings.Join([]string{header, strings.Join(rows, "\n"), footer}, "\n")
}

func (m Model) Header() string {
	paneName := "SELECT"
	if m.mode == ModeDashboard {
		paneName = "Dashboard"
	} else if m.activePane == 1 {
		paneName = "Single"
	}
	selectedCount := len(m.selected[m.activePane])
	text := fmt.Sprintf("%s | selected:%d | Tab/Shift+Tab roots | Up/Down rows | Space select | Enter inspect | f filter | ? help | Esc quit", paneName, selectedCount)
	return renderFullWidthBar(headerStyle, text, m.width)
}

func (m Model) filterBar() string {
	prompt := "Filter: " + m.filterQuery
	if m.filterMode {
		prompt += "_"
	}
	if strings.TrimSpace(m.filterQuery) == "" {
		prompt = "Filter: (press f to type, Enter apply, Esc close, Ctrl+U clear)"
	}
	return renderFullWidthBar(footerStyle, prompt, m.width)
}

func renderFullWidthBar(style lipgloss.Style, text string, width int) string {
	if width <= 0 {
		return style.Render(text)
	}
	contentWidth := maxInt(0, width-2)
	content := pad(truncate(text, contentWidth), contentWidth)
	return style.Render(content)
}

func (m Model) Footer() string {
	r := m.currentRow()
	if r == nil {
		return renderFullWidthBar(footerStyle, "No items", m.width)
	}
	footer := footerSummary(m.context, m.namespace, r)
	row := renderFullWidthBar(footerStyle, footer, m.width)
	if m.footerHeight == 1 {
		return row
	}

	rows := []string{row}
	for i := 1; i < m.footerHeight; i++ {
		rows = append(rows, renderFullWidthBar(footerStyle, "", m.width))
	}
	return strings.Join(rows, "\n")
}

func footerSummary(context, namespace string, r *Row) string {
	if r == nil {
		return "No items"
	}

	base := fmt.Sprintf("%s/%s | %s %s [%s]", context, namespace, r.Type, r.Name, r.Status)
	return fmt.Sprintf("%s key=%s", base, r.Key)
}

func (m Model) ViewRows(height int) string {
	rows := m.rowsByPane[m.activePane]
	if len(rows) == 0 {
		return strings.Repeat("\n", maxInt(0, height-1)) + "No items"
	}

	cursor := clamp(m.cursorByPane[m.activePane], 0, len(rows)-1)
	start := rowWindowStart(len(rows), height, cursor)
	end := minInt(len(rows), start+height)

	newRows := make([]string, 0, height)
	for i := start; i < end; i++ {
		newRows = append(newRows, m.renderRow(rows[i], i == cursor))
	}
	for len(newRows) < height {
		newRows = append(newRows, "")
	}
	return strings.Join(newRows, "\n")
}

func rowWindowStart(rowsLen, height, cursor int) int {
	if rowsLen <= 0 || height <= 0 {
		return 0
	}
	maxStart := maxInt(0, rowsLen-height)
	start := maxInt(0, cursor-height/2)
	end := minInt(rowsLen, start+height)
	if end-start < height {
		start = maxInt(0, end-height)
	}
	return clamp(start, 0, maxStart)
}

func (m Model) headerText() string {
	headerText := fmt.Sprintf("FILE | %s | Esc close", m.view.Title)
	if m.view.Kind == FileHelp {
		return "HELP | Keybindings | Esc close"
	}
	if m.view.SearchMode {
		headerText = "FILE | SEARCH | Enter apply | Esc cancel"
	}
	if m.view.Kind != FileYAML {
		return headerText
	}

	lineInfo := fmt.Sprintf("line %d/%d col %d", minInt(len(m.view.Rows), m.view.Scroll+1), len(m.view.Rows), m.view.ColScroll+1)
	if len(m.view.MatchRows) > 0 {
		lineInfo = fmt.Sprintf("%s | match %d/%d", lineInfo, m.view.ActiveMatch+1, len(m.view.MatchRows))
	}
	return fmt.Sprintf("%s | %s", headerText, lineInfo)
}

func (m Model) footerText() string {
	footerText := "Up/Down scroll | PgUp/PgDn page | g/G top/bottom | Left/Right pan | Home/End line start/end | / search | n/N next/prev | y copy | e edit | Esc close"
	if m.view.Kind == FileHelp {
		footerText = "Tab/Shift+Tab panes | arrows rows | Space select | Enter inspect | Esc close"
	} else if m.view.SearchMode {
		footerText = "Search: " + m.view.SearchQuery
	} else if len(m.view.MatchRows) > 0 {
		footerText = fmt.Sprintf("%s | match %d/%d", footerText, m.view.ActiveMatch+1, len(m.view.MatchRows))
	}
	if m.view.ActionStatus != "" {
		footerText += " | " + m.view.ActionStatus
	}
	return footerText
}

func (m Model) renderRows(rowsHeight int) []string {
	start := clamp(m.view.Scroll, 0, maxInt(0, len(m.view.Rows)-1))
	end := minInt(len(m.view.Rows), start+rowsHeight)
	rowsRows := make([]string, 0, rowsHeight)
	lineNumberWidth := len(fmt.Sprintf("%d", maxInt(1, len(m.view.Rows))))
	contentWidth := maxInt(1, m.width-lineNumberWidth-4)
	activeMatchRow := m.view.activeMatchRow()

	for i := start; i < end; i++ {
		line := visibleSegment(m.view.Rows[i], m.view.ColScroll, contentWidth)
		line = highlightSearchTerm(line, m.view.SearchQuery, i == activeMatchRow)
		line = rowStyle(line, i, m.view.MatchRows, activeMatchRow)
		prefix := rowPrefix(i, lineNumberWidth, m.view.MatchRows, activeMatchRow)
		rowsRows = append(rowsRows, prefix+line)
	}
	return rowsRows
}

func (m Model) renderRow(r Row, fileed bool) string {
	if r.Separator {
		return ""
	}
	state := rowState{Focused: fileed, Selected: m.selected[m.activePane][r.Key]}
	rowContent := rowContent(r, state)
	content := withSelectionMarkerOnRow(rowContent, rowSelectionMarker(state))
	return m.styleRowContent(content, state)
}

func rowSelectionMarker(state rowState) string {
	if state.Selected {
		return "⚠️"
	}
	return "[ ]"
}

func rowContent(r Row, state rowState) string {
	rowContent := r.Text
	if state.Focused || state.Selected {
		rowContent = r.Plain
		if rowContent == "" {
			rowContent = r.PlainText
		}
	}
	if strings.TrimSpace(rowContent) == "" {
		return r.Name
	}
	return rowContent
}

func (m Model) styleRowContent(content string, state rowState) string {
	if !state.Focused && !state.Selected {
		return content
	}

	width := maxInt(1, m.width)
	content = pad(truncate(content, width), width)
	if state.Focused {
		return fileedCell.Render(content)
	}
	return selectedRowStyle.Render(content)
}

func withSelectionMarkerOnRow(rowContent, marker string) string {
	for _, branch := range []string{"├─ ", "└─ "} {
		if idx := strings.Index(rowContent, branch); idx >= 0 {
			insertPos := idx + len(branch)
			tail := rowContent[insertPos:]
			if marker == "[ ]" {
				if emoji, rest, ok := consumeLeadingEmoji(tail); ok {
					return rowContent[:insertPos] + emoji + " " + rest
				}
			}
			return rowContent[:insertPos] + marker + " " + tail
		}
	}
	if marker == "[ ]" {
		if emoji, rest, ok := consumeLeadingEmoji(rowContent); ok {
			return emoji + " " + rest
		}
	}
	return marker + " " + rowContent
}

func consumeLeadingEmoji(s string) (string, string, bool) {
	trimmed := strings.TrimLeft(s, " ")
	if trimmed == "" {
		return "", s, false
	}
	r, size := utf8.DecodeRuneInString(trimmed)
	if r == utf8.RuneError || r <= unicode.MaxASCII || unicode.IsLetter(r) || unicode.IsNumber(r) {
		return "", s, false
	}
	rest := strings.TrimLeft(trimmed[size:], " ")
	return string(r), rest, true
}
