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

	if m.file != nil {
		return m.viewFile()
	}

	header := m.Header()
	footer := m.Footer()
	rowsHeight := maxInt(1, m.height-1-m.footerHeight)
	rows := m.ViewRows(rowsHeight)

	return strings.Join([]string{header, rows, footer}, "\n")
}

func (m Model) Header() string {
	paneName := "SELECT"
	if m.mode == ModeDashboard {
		paneName = "Dashboard"
	} else if m.activePane == 1 {
		paneName = "Single"
	}
	selectedCount := len(m.selected[m.activePane])
	text := fmt.Sprintf("%s | selected:%d | Tab pane | Up/Down rows | Space select | Enter inspect | ? help | Esc quit", paneName, selectedCount)
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

func (m Model) viewFile() string {
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

func (m Model) fileHeaderText() string {
	headerText := fmt.Sprintf("FILE | %s | Esc close", m.file.Title)
	if m.file.Kind == FileHelp {
		return "HELP | Keybindings | Esc close"
	}
	if m.file.SearchMode {
		headerText = "FILE | SEARCH | Enter apply | Esc cancel"
	}
	if m.file.Kind != FileYAML {
		return headerText
	}

	lineInfo := fmt.Sprintf("line %d/%d col %d", minInt(len(m.file.Lines), m.file.Scroll+1), len(m.file.Lines), m.file.ColScroll+1)
	if len(m.file.MatchLines) > 0 {
		lineInfo = fmt.Sprintf("%s | match %d/%d", lineInfo, m.file.ActiveMatch+1, len(m.file.MatchLines))
	}
	return fmt.Sprintf("%s | %s", headerText, lineInfo)
}

func (m Model) fileFooterText() string {
	footerText := "Up/Down scroll | PgUp/PgDn page | g/G top/bottom | Left/Right pan | Home/End line start/end | / search | n/N next/prev | y copy | e edit | Esc close"
	if m.file.Kind == FileHelp {
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

func (m Model) fileRowsLines(rowsHeight int) []string {
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

func (m Model) renderRow(r Row, fileed bool) string {
	state := rowRenderState{Focused: fileed, Selected: m.selected[m.activePane][r.Key]}
	rowContent := rowContent(r, state)
	content := withSelectionMarkerOnRow(rowContent, rowSelectionMarker(state))
	return m.styleRowContent(content, state)
}

func rowSelectionMarker(state rowRenderState) string {
	if state.Selected {
		return "⚠️"
	}
	return "[ ]"
}

func rowContent(r Row, state rowRenderState) string {
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

func (m Model) styleRowContent(content string, state rowRenderState) string {
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
					return rowContent[:insertPos] + "" + emoji + " " + rest
				}
			}
			return rowContent[:insertPos] + marker + " " + tail
		}
	}
	if marker == "[ ]" {
		if emoji, rest, ok := consumeLeadingEmoji(rowContent); ok {
			return "[" + emoji + "] " + rest
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
